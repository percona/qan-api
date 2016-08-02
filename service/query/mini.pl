#!/usr/bin/env perl

# Copyright (c) 2015, Percona LLC and/or its affiliates. All rights reserved.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>

use strict;
use warnings FATAL => 'all';

BEGIN {
   $INC{$_} = __FILE__ for map { (my $pkg = "$_.pm") =~ s!::!/!g; $pkg } (qw(
      Percona::Query::Mini
   ));
}

package Percona::Query::Mini;

use strict;
use warnings FATAL => 'all';
use English qw(-no_match_vars);
use constant PTDEBUG => $ENV{PTDEBUG} || 0;

# An incomplete list of verbs that can appear in queries.
our $verbs   = qr{^SHOW|^FLUSH|^COMMIT|^ROLLBACK|^BEGIN|SELECT|INSERT
                  |UPDATE|DELETE|REPLACE|^SET|UNION|^START|^LOCK}xi;

# The one-line comment pattern is quite crude.  This is intentional for
# performance.  The multi-line pattern does not match version-comments.
my $olc_re = qr/(?:--|#)[^'"\r\n]*(?=[\r\n]|\Z)/;  # One-line comments
my $mlc_re = qr#/\*[^!].*?\*/#sm;                  # But not /*!version */
my $vlc_re = qr#/\*.*?[?0-9]+.*?\*/#sm;            # For SHOW + /*!version */
my $vlc_rf = qr#^SHOW\s+/\*![?0-9]+(.*?)\*/#i;     # Variation for SHOW

my $tbl_ident = qr/(?:`[^`]+`|\w+)(?:\.(?:`[^`]+`|\w+))?/;

# This regex finds things that look like database.table identifiers, based on
# their proximity to keywords.  (?<!KEY\s) is a workaround for ON DUPLICATE KEY
# UPDATE, which is usually followed by a column name.
our $tbl_regex = qr{
         \b(?:FROM|JOIN|(?<!KEY\s)UPDATE|INTO) # Words that precede table names
         \b\s*
         \(?                                   # Optional paren around tables
         # Capture the identifier and any number of comma-join identifiers that
         # follow it, optionally with aliases with or without the AS keyword
         ($tbl_ident
            (?: (?:\s+ (?:AS\s+)? \w+)?, \s*$tbl_ident )*
         )
}xio;

# http://dev.mysql.com/doc/refman/5.1/en/sql-syntax-data-definition.html
# We treat TRUNCATE as a dds but really it's a data manipulation statement.
my $data_def_stmts = qr/(?:CREATE|ALTER|TRUNCATE|DROP|RENAME)/i;

# http://dev.mysql.com/doc/refman/5.1/en/sql-syntax-data-manipulation.html
# Data manipulation statements.
my $data_manip_stmts = qr/(?:INSERT|UPDATE|DELETE|REPLACE)/i;

sub get_tables {
   my ( $query ) = @_;
   return unless $query;
   PTDEBUG && _d('Getting tables for', $query);

   # Handle CREATE, ALTER, TRUNCATE and DROP TABLE.
   my ( $ddl_stmt ) = $query =~ m/^\s*($data_def_stmts)\b/i;
   if ( $ddl_stmt ) {
      PTDEBUG && _d('Special table type:', $ddl_stmt);
      $query =~ s/IF\s+(?:NOT\s+)?EXISTS//i;
      if ( $query =~ m/$ddl_stmt DATABASE\b/i ) {
         # Handles CREATE DATABASE, not to be confused with CREATE TABLE.
         PTDEBUG && _d('Query alters a database, not a table');
         return ();
      }
      if ( $ddl_stmt =~ m/CREATE/i && $query =~ m/$ddl_stmt\b.+?\bSELECT\b/i ) {
         # Handle CREATE TABLE ... SELECT.  In this case, the real tables
         # come from the SELECT, not the CREATE.
         my ($select) = $query =~ m/\b(SELECT\b.+)/is;
         PTDEBUG && _d('CREATE TABLE ... SELECT:', $select);
         return get_tables($select);
      }
      my ($tbl) = $query =~ m/TABLE\s+($tbl_ident)(\s+.*)?/i;
      if ( !$tbl ) {
         # Perf schema: TRUNCATE db.tbl
         ($tbl) = $query =~ m/$ddl_stmt\s+($tbl_ident)/i;
      }
      PTDEBUG && _d('Matches table:', $tbl);
      return ($tbl);
   }

   # These keywords may appear between UPDATE or SELECT and the table refs.
   # They need to be removed so that they are not mistaken for tables.
   $query =~ s/(?:LOW_PRIORITY|IGNORE|STRAIGHT_JOIN|DELAYED)\s+/ /ig;

   # Another special case: LOCK TABLES tbl [[AS] alias] READ|WRITE, etc.
   # We strip the LOCK TABLES stuff and append "FROM" to fake a SELECT
   # statement and allow $tbl_regex to match below.
   if ( $query =~ s/^\s*LOCK TABLES\s+//i ) {
      PTDEBUG && _d('Special table type: LOCK TABLES');
      $query =~ s/\s+(?:READ(?:\s+LOCAL)?|WRITE)\s*//gi;
      PTDEBUG && _d('Locked tables:', $query);
      $query = "FROM $query";
   }

   $query =~ s/\\["']//g;   # quoted strings
   $query =~ s/".*?"/?/sg;  # quoted strings
   $query =~ s/'.*?'/?/sg;  # quoted strings

   # INSERT and REPLACE without INTO
   # https://bugs.launchpad.net/percona-toolkit/+bug/984053
   if ( $query =~ m/\A\s*(?:INSERT|REPLACE)(?!\s+INTO)/i ) {
      # Add INTO so the reset of the code work as usual.
      $query =~ s/\A\s*((?:INSERT|REPLACE))\s+/$1 INTO /i;
   }

   if ( $query =~ m/\A\s*LOAD DATA/i ) {
      my ($tbl) = $query =~ m/INTO TABLE\s+(\S+)/i;
      return $tbl;
   }

   my @tables;
   foreach my $tbls ( $query =~ m/$tbl_regex/gio ) {
      PTDEBUG && _d('Match tables:', $tbls);

      # Some queries coming from certain ORM systems will have superfluous
      # parens around table names, like SELECT * FROM (`mytable`);  We match
      # these so the table names can be extracted more simply with regexes.  But
      # in case of subqueries, this can cause us to match SELECT as a table
      # name, for example, in SELECT * FROM (SELECT ....) AS X;  It's possible
      # that SELECT is really a table name, but so unlikely that we just skip
      # this case.
      next if $tbls =~ m/\ASELECT\b/i;

      foreach my $tbl ( split(',', $tbls) ) {
         # Remove implicit or explicit (AS) alias.
         $tbl =~ s/\s*($tbl_ident)(\s+.*)?/$1/gio;

         # Sanity check for cases like when a column is named `from`
         # and the regex matches junk.  Instead of complex regex to
         # match around these rarities, this simple check will save us.
         if ( $tbl !~ m/[a-zA-Z]/ ) {
            PTDEBUG && _d('Skipping suspicious table name:', $tbl);
            next;
         }

         push @tables, $tbl;
      }
   }
   return @tables;
}

# Strips comments out of queries.
sub strip_comments {
   my ( $query ) = @_;
   return unless $query;
   $query =~ s/$mlc_re//go;
   $query =~ s/$olc_re//go;
   if ( $query =~ m/$vlc_rf/ ) { # contains show + version
      my $qualifier = $1 || '';
      $query =~ s/$vlc_re/$qualifier/go;
   }
   return $query;
}

# Gets the verbs from an SQL query, such as SELECT, UPDATE, etc.
sub distill_verbs {
   my ( $query ) = @_;

   # Simple verbs that normally don't have comments, extra clauses, etc.
   $query =~ m/\A\s*call\s+([^\(]+)/i  && return "CALL $1";
   $query =~ m/\A\s*use\s+/i           && return "USE";
   $query =~ m/\A\s*UNLOCK TABLES/i    && return "UNLOCK";
   $query =~ m/\A\s*xa\s+(\S+)/i       && return "XA_$1";
   $query =~ m/\A(\S+)\Z/              && return uc($1);  # single word
   $query =~ m/\A\s*(OPTIMIZE|CHECKSUM)\s+TABLE\s+(\S+)/i && return uc($1) . " " . $2;
   $query =~ m/\A\s*(GRANT|REVOKE)\s/i              && return uc($1);
   $query =~ m/\A\s*(RELEASE)?\s*SAVEPOINT\s/i      && return $1 ? "RELEASE SAVEPOINT" : "SAVEPOINT";
   $query =~ m/\A\s*KILL\s+(CONNECTION|QUERY)?\s*/i && return "KILL" . ($1 ?  uc(" $1") : "");
   $query =~ m/\A\s*EXPLAIN\s+/i                    && return "EXPLAIN";

   if ( $query =~ m/\A\s*LOAD/i ) {
      my ($tbl) = $query =~ m/INTO TABLE\s+(\S+)/i;
      $tbl ||= '';
      $tbl =~ s/`//g;
      return "LOAD DATA $tbl";
   }

   if ( $query =~ m/\Aadministrator command:/ ) {
      $query =~ s/administrator command:/ADMIN/;
      $query = uc $query;
      return $query;
   }

   # All other, more complex verbs. 
   $query = strip_comments($query);

   # SHOW statements are either 2 or 3 words: SHOW A (B), where A and B
   # are words; B is optional.  E.g. "SHOW TABLES" or "SHOW SLAVE STATUS". 
   # There's a few common keywords that may show up in place of A, so we
   # remove them first.  Then there's some keywords that signify extra clauses
   # that may show up in place of B and since these clauses are at the
   # end of the statement, we remove everything from the clause onward.
   if ( $query =~ m/\A\s*SHOW\s+/i ) {
      PTDEBUG && _d($query);

      # Remove common keywords.
      $query = uc $query;
      $query =~ s/\s+(?:SESSION|FULL|STORAGE|ENGINE)\b/ /g;
      # This should be in the regex above but Perl doesn't seem to match
      # COUNT\(.+\) properly when it's grouped.
      $query =~ s/\s+COUNT[^)]+\)//g;

      # Remove clause keywords and everything after.
      $query =~ s/\s+(?:FOR|FROM|LIKE|WHERE|LIMIT|IN)\b.+//ms;

      # The query should now be like SHOW A B C ... delete everything after B,
      # collapse whitespace, and we're done.
      $query =~ s/\A(SHOW(?:\s+\S+){1,2}).*\Z/$1/s;
      $query =~ s/\s+/ /g;
      PTDEBUG && _d($query);
      return $query;
   }
   
   if ( $query =~ m/\A\s*DESC(?:RIBE)?\s+($tbl_ident)/i ) {
        my $tbl = $1 || '';
        $tbl =~ s/^`//;
        $tbl =~ s/`$//;
        return "DESCRIBE $tbl"
   }

   # Data defintion statements verbs like CREATE and ALTER.
   # The two evals are a hack to keep Perl from warning that
   # "QueryParser::data_def_stmts" used only once: possible typo at...".
   # Some day we'll group all our common regex together in a packet and
   # export/import them properly.
   my ( $dds ) = $query =~ /^\s*($data_def_stmts)\b/i;
   if ( $dds) {
      # https://bugs.launchpad.net/percona-toolkit/+bug/821690
      $query =~ s/\s+IF(?:\s+NOT)?\s+EXISTS/ /i;
      my ( $obj ) = $query =~ m/$dds.+(DATABASE|TABLE)\b/i;
      $obj = uc $obj if $obj;
      PTDEBUG && _d('Data def statment:', $dds, 'obj:', $obj);
      my ($db_or_tbl)
         = $query =~ m/(?:TABLE|DATABASE)\s+($tbl_ident)(\s+.*)?/i;
      PTDEBUG && _d('Matches db or table:', $db_or_tbl);
      return uc($dds . ($obj ? " $obj" : '')), $db_or_tbl;
   }

   # All other verbs, like SELECT, INSERT, UPDATE, etc.  First, get
   # the query type -- just extract all the verbs and collapse them
   # together.
   my @verbs = $query =~ m/\b($verbs)\b/gio;
   @verbs    = do {
      my $last = '';
      grep { my $pass = $_ ne $last; $last = $_; $pass } map { uc } @verbs;
   };

   # http://code.google.com/p/maatkit/issues/detail?id=1176
   # A SELECT query can't have any verbs other than UNION.
   # Subqueries (SELECT SELECT) are reduced to 1 SELECT in the
   # do loop above.  And there's no valid SQL syntax like
   # SELECT ... DELETE (there are valid multi-verb syntaxes, like
   # INSERT ... SELECT).  So if it's a SELECT with multiple verbs,
   # we need to check it else SELECT c FROM t WHERE col='delete'
   # will incorrectly distill as SELECT DELETE t.
   if ( ($verbs[0] || '') eq 'SELECT' && @verbs > 1 ) {
      PTDEBUG && _d("False-positive verbs after SELECT:", @verbs[1..$#verbs]);
      my $union = grep { $_ eq 'UNION' } @verbs;
      @verbs    = $union ? qw(SELECT UNION) : qw(SELECT);
   }

   # This used to be "my $verbs" but older verisons of Perl complain that
   # ""my" variable $verbs masks earlier declaration in same scope" where
   # the earlier declaration is our $verbs.
   # http://code.google.com/p/maatkit/issues/detail?id=957
   my $verb_str = join(q{ }, @verbs);
   if ($verb_str eq "") {
      my @verbs_arr = split(" ", $query);
      if (scalar @verbs_arr > 0) {
         if ( $query =~ m/\A(\S+)\s+(\S+)\Z/ ) {
            $verb_str = uc($1) . " " . uc($2); # two words
         }
         else {
           $verb_str = sprintf("%s (?)", uc($verbs_arr[0]));
         }
      }
   }

   return $verb_str;
}

sub distill_tables {
   my ( $query, $table, %args ) = @_;

   # Perf schema: `db` . `tbl` -> db.tbl
   $query =~ s/`(\S+)` . `(\S+)`/$1.$2/g;

   # "Fingerprint" the tables.
   my @tables = map {
      $_ =~ s/`//g;
      # Don't replace numbers in table names to match Vitess fingerprinting method.
      #$_ =~ s/(_?)[0-9]+/$1?/g;
      $_;
   } grep { defined $_ } get_tables($query);

   push @tables, $table if $table;

   # Collapse the table list
   @tables = do {
      my $last = '';
      grep { my $pass = $_ ne $last; $last = $_; $pass } @tables;
   };

   return @tables;
}

sub mini {
   my ( $query ) = @_;

   chomp($query);
   if (!defined $query || $query =~ m/^\s*\z/) {
      return "";
   }

   # distill_verbs() may return a table if it's a special statement
   # like TRUNCATE TABLE foo.  distill_tables() handles some but not
   # all special statements so we pass the special table from distill_verbs()
   # to distill_tables() in case it's a statement that the latter
   # can't handle.  If it can handle it, it will eliminate any duplicate
   # tables.
   my ($verbs, $table) = distill_verbs($query);

   if ( $verbs && $verbs =~ m/^SHOW/ ) {
      # Ignore tables for SHOW statements and normalize some
      # aliases like SCHMEA==DATABASE, KEYS==INDEX.
      my %alias_for = qw(
         SCHEMA   DATABASE
         KEYS     INDEX
         INDEXES  INDEX
      );
      map { $verbs =~ s/$_/$alias_for{$_}/ } keys %alias_for;
      $query = $verbs;
   }
   elsif ( $verbs && ($verbs =~ m/^(?:LOAD DATA|REVOKE|EXPLAIN)/) ) {
      return $verbs;
   }
   else {
      # For everything else, distill the tables.
      my @tables = distill_tables($query, $table);
      $query     = join(q{ }, $verbs, @tables); 
   } 

   return $query;
}

1;

package main;

use strict;
use warnings FATAL => 'all';
use English qw(-no_match_vars);
use constant PTDEBUG => $ENV{PTDEBUG} || 0;

$OUTPUT_AUTOFLUSH = 1;

sub main {
   local @ARGV = @_;
   my $mini = "";
   while(my $query = <STDIN>) {
      $mini = Percona::Query::Mini::mini($query);
      print "$mini\n";
   }
   return 0;
}

if ( !caller ) { exit main(@ARGV); }

1;

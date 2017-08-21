# lexes

A LexisNexis Boolean query to Elasticsearch query parser.

Operator precedence
--------------------
Operators are in the following precedence:
1. OR
2. w/n
3. w/s
4. w/p
5. AND 
6. NOT 

For example, given a query:

a OR b AND c OR d

the clause, "a OR b", is parsed first. The clause, "c OR d", is parsed next. And, then the "AND" clause is parsed, combining the two previous clauses.  

Multiple operators of the same precedence are parsed left to right.

Proximity Operators
--------------------
Unlike Lexis, w/s and w/p can be used in combinination with w/n operators. This is because, w/s and w/p default to a proximity of 20 and 50 terms respectively. 

Proximity operators do not work in conjunction with boolean operators in Elasticsearch. To get around this, a span_near query is used with a slop of 1,000,000 to represent an AND operator, and span_or represents an OR operator.  

/n operators are parsed from smallest to largest, the same as done by Lexis(see https://www.lexisnexis.com/help/global/US/en_US/gh_terms.asp). 

Not Operators
--------------------
"NOT" is supported both as "a AND NOT b" (Lexis format), or by simply specifying "a NOT b". 

Query terms 
--------------------
A term is a term in a document. Terms are matched exactly; no stemming occurs by default. For example "contract" will match documents containing the term "contract", but it will not match "contractual", "contra" or "contracted". 

An option exists for specifying whether query terms are automatically separated by an operator (by default OR). However, it can be specified that a query that contains terms not separated by operators return an error.  

Wildcard operators are supported in clauses that do not involve proximity operators. Both "*" and "!" are supported as wildcard operators. Two instances are applicable:
1. "car*" or "car!" - in which case, the term, "car", denotes a prefix, and any terms that have the term as its prefix are matched - ie. car, carried, carrier;
2. "car*d" - in which case, the asterisk denotes any number of characters that may be matched before the prefix and suffix are to be matched.

In progress
--------------------
- PRE/n operators.
- NOT w/n and NOT PRE/n operators.
- W/SEG operators. There is no need to currently support this as the indexed data does not contain recognised segments. 
- Stemming and searching on stems.
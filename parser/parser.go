package parser

import (
	"errors"
	"reflect"
	"strings"
	"encoding/json"

	"log"
)

const FALSE_SPAN_AND int = 1000000

const WITHIN_PARA string = "50"
const WITHIN_SENT string = "20"

type ttype uint8
const (
	Phrase ttype = iota
	Prefix
	Wildcard
	NotString
)

type Node struct {
	Operator string

	Children []interface{}

	Proximity string

	Slop bool

	Type []ttype
}

func ParseJson(query, field string, retrieve []string, insert_ops, highlight bool) ([]byte, error) {
	qry, err := Parse(query, field, retrieve, insert_ops, highlight)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(qry, "", "    ")
}

// Parse a boolean query from string. In the process, the validity of the query is checked.
func Parse(query, field string, retrieve []string, insert_ops, highlight bool) (*map[string]interface{}, error) {
	tree, err := ParseTree(query, field, retrieve, insert_ops)
	if err != nil {
		return nil, err
	}
	log.Println(tree)
	return parseToMap(tree, field, retrieve, highlight), nil
}


func ParseTree(query, field string, retrieve []string, insert_ops bool) (*Node, error) {
	query = strings.Replace(query, ".", " ", -1)
	query = strings.TrimSpace(query)
	query = strings.ToLower(query)

	stack, err := createStack(query)
	if err != nil {
		return nil, err
	}
	stack = removeLexisAndNot(stack)
	stack, err = checkKeywordArrangement(stack, insert_ops)
	if err != nil {
		return nil, err
	}

	res, err := convertInfixToPostfix(stack)
	if err != nil {
		return nil, err
	}
	return parsePostfix(res)
}

func parsePostfix(rpn_stack []string) (*Node, error) {
	stack := make([]interface{}, 0)
	num_take := 2

	for i := range rpn_stack {
		token := rpn_stack[i]

		if isOperator(token) {
			
			node := Node{}
			operands := make([]interface{}, 0)

			operands, stack = stack[len(stack) - num_take:], stack[:len(stack) - num_take]

			switch(token) {
				case "w/p":
					node.Slop = true
					node.Proximity = WITHIN_PARA
					node.Operator = "must"
					break

				case "w/s":
					node.Slop = true
					node.Proximity = WITHIN_SENT
					node.Operator = "must"
					break

				case "and":
					node.Operator = "must"
					break

				case "or":
					node.Operator = "should"
					break

				case "not": 
					node.Operator = "must_not"
					break

				default:
					// Handle w/n
					node.Slop = true
					node.Proximity = token[2:]
					node.Operator = "must"
					break
			}

			for op := range operands {
				if reflect.TypeOf(operands[op]).Kind() == reflect.String {
					node.Type = append(node.Type, assignTtype(operands[op].(string)))
				} else {
					node.Type = append(node.Type, NotString)
				}

				if node.Type[op] == Prefix {
					operands[op] = strings.TrimSuffix(operands[op].(string), "*")
				}
				node.Children = append(node.Children, operands[op])
			}
			// }
			stack = append(stack, node)

		} else {
			stack = append(stack, token)
		}
	}

	if len(stack) != 1 {
		return nil, errors.New("Something has gone wrong in the stack creation.")
	} else if reflect.TypeOf(stack[0]).Kind() == reflect.String {
		stack[0] = Node{
			Operator: "must",
			Children: []interface{}{stack[0]},
			Type: []ttype{assignTtype(stack[0].(string))},
		}
	}
	n := stack[0].(Node)
	return &n, nil
}

func assignTtype(token string) ttype {
	if token[len(token)-1] == '*' {
		return Prefix
	} else if strings.Contains(token, "*") {
		return Wildcard
	}
	return Phrase
}

func nodeToInterface(n Node, field string, span_child bool) interface{} {
	var clause interface{}

	if n.Slop || span_child {
		clause = handleSpanOperator(n, field)
	} else {
		children := make([]interface{}, len(n.Children))

		for i := range n.Children {
			if reflect.TypeOf(n.Children[i]).Kind() == reflect.String {
				children[i] = parseTerm(n.Children[i].(string), field, n.Type[i], span_child)
			} else {
				// Parse node recursively
				children[i] = nodeToInterface(n.Children[i].(Node), field, span_child)
			}
		}

		if n.Operator == "must_not" {
			clause = map[string]interface{} {
				"bool" : map[string]interface{} {
					"must_not": children[0],
					"must": children[1],
				},	
			}
		} else {
			clause = map[string]interface{} {
				"bool" : map[string]interface{} {
					n.Operator : children,
				},	
			}
		}
		
	}

	return clause
}

func handleSpanOperator(n Node, field string) *map[string]interface{} {
	clauses := make([]interface{}, len(n.Children))

	for i := range n.Children {
		if reflect.TypeOf(n.Children[i]).Kind() == reflect.String {
			clauses[i] = parseTerm(n.Children[i].(string), field, n.Type[i], true)
		} else {
			// Parse node recursively
			clauses[i] = nodeToInterface(n.Children[i].(Node), field, true)
		}
	}

	node := map[string]interface{}{}
	if n.Slop {
		node = map[string]interface{}{
			"span_near" : map[string]interface{}{
				"clauses" : clauses,
				"slop" : n.Proximity,
				"in_order" : false,
			},
		}
	} else if n.Operator == "must" {
		node = map[string]interface{}{
			"span_near" : map[string]interface{}{
				"clauses" : clauses,
				"slop" : FALSE_SPAN_AND,
				"in_order" : false,
			},
		}
	} else if n.Operator == "should" {
		node = map[string]interface{}{
			"span_or" : map[string]interface{}{
				"clauses" : clauses,
				"in_order" : false,
			},
		}
	} else if n.Operator == "must_not" {
		node = map[string]interface{} {
			"span_not" : map[string]interface{}{
				"include" : clauses[1],
				"exclude" : clauses[0],			
			},
		}
	}

	return &node
}

func parseTerm(term, field string, t ttype, span bool) *map[string]interface{} {

	clause := map[string]interface{}{}

	if span {
		terms := strings.Split(term, " ")
		numTerms := len(terms)
		if numTerms > 1 {
			spanTerms := make([]map[string]interface{}, numTerms)
			for i := range terms {
				if i == numTerms - 1 && t == Prefix {
					spanTerms[i] = map[string]interface{}{
						"span_multi" : map[string]interface{}{
							"match" : map[string]interface{}{
								"prefix" : map[string]interface{}{
									field : terms[i],
								},
							},
						},
					}
				} else if i == numTerms - 1 && t == Wildcard {
					spanTerms[i] = map[string]interface{}{
						"span_multi" : map[string]interface{}{
							"match" : map[string]interface{}{
								"wildcard" : map[string]interface{}{
									field : terms[i],
								},
							},
						},
					}
				} else {
					spanTerms[i] = map[string]interface{}{
						"span_term" : map[string]interface{}{
							field: terms[i],
						},
					}
				}
			}
			clause = map[string]interface{}{
				"span_near" : map[string]interface{}{
					"clauses": spanTerms,
					"in_order": true,
					"slop": 0,
				},
			}
		} else {
			switch(t) {
				case Phrase:
					clause = map[string]interface{}{
						"span_term" : map[string]interface{}{
							field : term,
						},
					}
					break

				case Prefix:
					clause = map[string]interface{}{
						"span_multi" : map[string]interface{}{
							"match" : map[string]interface{}{
								"prefix" : map[string]interface{}{
									field : term,
								},
							},
						},
					}
					break

				case Wildcard:
					clause = map[string]interface{}{
						"span_multi" : map[string]interface{}{
							"match" : map[string]interface{}{
								"wildcard" : map[string]interface{}{
									field : term,
								},
							},
						},
					}
					break
			}
		}
	} else {
		switch(t) {
			case Phrase:
				clause = map[string]interface{}{
					"match_phrase" : map[string]interface{}{
						field : term,
					},
				}
				break

			case Prefix:
				clause = map[string]interface{}{
					"match_phrase_prefix" : map[string]interface{}{
						field : term,
					},
				}
				break

			case Wildcard:
				clause = map[string]interface{}{
					"wildcard" : map[string]interface{}{
						field : term,
					},
				}
				break
		}
	}

	return &clause
}

func parseToMap(n *Node, field string, retrieve []string, highlight bool) *map[string]interface{} {

	query := nodeToInterface(*n, field, false)

	res := map[string]interface{}{
		"query" : query,
	}

	if len(retrieve) != 0 {
		res["_source"] = retrieve
	}

	if highlight {
		res["highlight"] = map[string]interface{}{
			"order" : "score",
			"fields" : map[string]interface{}{
				"plain_text" : map[string]interface{}{
					"number_of_fragments": 3,
				},
			},
		}
	}
	return &res
}

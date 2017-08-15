package parser

import (
	"errors"
	"reflect"
	"strings"
	"encoding/json"
)

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


// Parse a boolean query from string. In the process, the validity of the query is checked. 
func Parse(query string, insert_ops bool, field string) ([]byte, error) {

	query = strings.Replace(query, ".", " ", -1)
	query = strings.TrimSpace(query)
	query = strings.ToLower(query)

	quote_pos := make([]int, 0)

	level := 0

	// Error checking parentheses and quotes. This is implicitly done in the Shunting Yard algorithm, 
	// but, it is done here to give explicit error reasons. 
	for i, char := range query { 
		if char == '"' {
			quote_pos = append(quote_pos, i)
		}
		if char == '(' && (len(quote_pos) % 2) == 0 {
			level++ 

		}
		if char == ')' && (len(quote_pos) % 2) == 0 {
			level--
		} 
		// Replace truncation to match es format.
		if char == '!' && (len(quote_pos) % 2) == 0 {
			char = '*'
		}

		if level < 0 {
			return []byte{}, errors.New("Malformed query: Parenthetical clauses do not match. A clause is closed prior to being opened.")
		}
	}
	if level != 0 {
		return []byte{}, errors.New("Malformed query: Number of parentheses does not match. A clause is not closed.")
	} 
	if (len(quote_pos) % 2) != 0 {
		return []byte{}, errors.New("Malformed query: Number of quotation marks does not match. A quotation is not closed.")
	}

	stack := createStack(query)
	stack = removeLexisAndNot(stack) 
	stack, err := checkKeywordArrangement(stack, insert_ops)
	if err != nil {
		return []byte{}, err
	}

	res, err := convertInfixToPostfix(stack)
	if err != nil {
		return []byte{}, err
	}
	tree, err := parsePostfix(res)
	if err != nil {
		return []byte{}, err
	}
	return parseToJson(&tree, field, false)
}

func parsePostfix(rpn_stack []string) (Node, error) {
	stack := make([]interface{}, 0)
	
	for i := range rpn_stack {
		token := rpn_stack[i]

		if isOperator(token) {
			num_take := 2

			node := Node{}
			operands := make([]interface{}, 0)

			operands, stack = stack[len(stack) - num_take:], stack[:len(stack) - num_take] 

			if token == "w/p" {
				node.Slop = true
				node.Proximity = WITHIN_PARA
				node.Operator = "must"
			} else if token == "w/s" {
				node.Slop = true
				node.Proximity = WITHIN_SENT
				node.Operator = "must"
			} else if token == "and" {
				node.Operator = "must"
			} else if token == "or" {
				node.Operator = "should"
			} else if token == "not" {
				node.Operator = "must"

				left := Node{
					Operator: "must",
					Children: []interface{}{operands[1]},
					Type: make([]ttype, 1),
 				}
				if reflect.TypeOf(operands[1]) == reflect.TypeOf("") {
					left.Type[0] = assignTtype(operands[1].(string))
				}

				right := Node{
					Operator : "must_not",
					Children: []interface{}{operands[0]},
					Type: make([]ttype, 1),
				}
				if reflect.TypeOf(operands[0]) == reflect.TypeOf("") {
					right.Type[0] = assignTtype(operands[0].(string))
				} 

				node.Children = append(node.Children, left, right)

			} else {
				node.Slop = true
				node.Proximity = token[2:]
				node.Operator = "must"
			}
			
			if token != "not" {
				for op := range operands {
					if reflect.TypeOf(operands[op]) == reflect.TypeOf("") {
						node.Type = append(node.Type, assignTtype(operands[op].(string)))
					} else {
						node.Type = append(node.Type, NotString)
					}
					node.Children = append(node.Children, operands[op])
				}
			}

			stack = append(stack, node)

		} else {
			stack = append(stack, token)
		}
	}

	if len(stack) != 1 {
		return Node{}, errors.New("Something has gone wrong in the stack creation.")
	} else if reflect.TypeOf(stack[0]) == reflect.TypeOf("") {
		stack[0] = Node{
			Operator: "must",
			Children: []interface{}{stack[0]},
			Type: []ttype{assignTtype(stack[0].(string))},
		}
	}
	return stack[0].(Node), nil
}

func assignTtype(token string) ttype {
	if token[len(token)-1] == '*' {
		return Prefix
	} else if strings.Contains(token, "*") {
		return Wildcard
	} 
	return Phrase
}

func nodeToJson(n Node, field string) interface{} {
	clause := map[string][]interface{}{}
	
	if n.Slop {
		children := len(n.Children)
		clauses := make([]interface{}, children)
		for i := 0; i < children; i++ {
			if reflect.TypeOf(n.Children[i]) == reflect.TypeOf("") {
				clauses[i] = map[string]interface{}{
					"span_term" : map[string]interface{}{
						field : n.Children[i],
					},
				}
			} else {
				// Parse node recursively 		
				clauses[i] = nodeToJson(n.Children[i].(Node), field)
			}
		}
		node := map[string]interface{}{
			"span_near" : map[string]interface{}{
				"clauses" : clauses, 
				"slop" : n.Proximity,
				"in_order" : false,
			},
		}
		clause[n.Operator] = append(clause[n.Operator], node)
	} else {
		for i := range n.Children {
			if reflect.TypeOf(n.Children[i]) == reflect.TypeOf("") {
				clause[n.Operator] = append(clause[n.Operator], parseTerm(n.Children[i].(string), n.Type[i], field))
			} else {
				// Parse node recursively 		
				clause[n.Operator] = append(clause[n.Operator], nodeToJson(n.Children[i].(Node), field))
			}
		}
	}

	return clause
}

func parseTerm(term string, t ttype, field string) *map[string]interface{} {

	clause := map[string]interface{}{}

	switch(t) {
		case Phrase:
			clause = map[string]interface{}{
				"term" : map[string]interface{}{
					field : term,
				},
			}
			break

		case Prefix:
			clause = map[string]interface{}{
				"prefix" : map[string]interface{}{
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

	return &clause
}

func parseToJson(n *Node, field string, highlight bool) ([]byte, error) {
	query := nodeToJson(*n, field)
	res := map[string]interface{}{
		"query" : map[string]interface{}{
			"bool" : query,
		},
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
	
	return json.MarshalIndent(res, "", "   ")
	// if err != nil {
	// 	return []byte{}, err
	// }
	// return js, nil
}
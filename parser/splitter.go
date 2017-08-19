package parser

import (
	"errors"
	"strings"
)

var precedence = map[string]int {
	"or": 6,
	"w/" : 5,
	"w/s": 4,
	"w/p": 3,
	"and": 2, 
	"not": 1,	
}

func createStack(component string) ([]string, error) {
	word := ""
	stack := []string{}

	num_quote_encountered := 0
	level := 0

	component_len := len(component) - 1

	for i, char := range component {
		switch(char) {
			case '"':
				num_quote_encountered++ 
			case '(':
				if num_quote_encountered % 2 == 0 {
					stack = append(stack, "(")
					word = ""
				}
				level++
			case ')':
				if num_quote_encountered % 2 == 0 {
					if len(word) > 0 {
						stack = append(stack, strings.TrimSpace(word))
						word = ""
					}
					stack = append(stack, ")")
				}
				level--
			case ' ':
				if num_quote_encountered % 2 != 0 {
					word += string(char)
				} else {
					if len(word) > 0 {
						stack = append(stack, strings.TrimSpace(word))
						word = ""
					}
				}
			case '!':
				word += "*"
			default: 
				word += string(char)
		}
		if i == component_len && word != "" {
			stack = append(stack, strings.TrimSpace(word))		
		}
		if level < 0 {
			return []string{}, errors.New("Malformed query: Parenthetical clauses do not match. A clause is closed prior to being opened.")
		}
	}
	if level != 0 {
		return []string{}, errors.New("Malformed query: Number of parentheses does not match. A clause is not closed.")
	} 
	if num_quote_encountered % 2 != 0 {
		return []string{}, errors.New("Malformed query: Number of quotation marks does not match. A quotation is not closed.")
	}
	return stack, nil
}

func convertInfixToPostfix(in_stack []string) ([]string, error) {
	temp := []string{}
	result := []string{}

	for i := len(in_stack) - 1; i >= 0; i-- {
		token := in_stack[i]

		if token == ")" {
			temp = append(temp, token)
		} else if token == "(" {
			for len(temp) > 0 {
				var t string
				t, temp = temp[len(temp) - 1], temp[:len(temp) - 1]
				if t == ")" {
					break
				}
				result = append(result, t)
			}
		} else {
			prox := checkProximityOperators(token)
			if prox == 1 {
				token = "w/"		
			} else if prox == 2 {
				return []string{}, errors.New("Malformed proximity operator.")
			}

			curr, ok := precedence[token]

			if !ok {
				result = append(result, token)		
			} else {
				for len(temp) > 0 && precedence[temp[len(temp) - 1]] > curr {
					var t string
					t, temp = temp[len(temp) - 1], temp[:len(temp) - 1]
					result = append(result, t)
				}
				if token == "w/" {
					temp = append(temp, in_stack[i])
				} else {
					temp = append(temp, token)
				}
			}
		}
	}

	for len(temp) > 0 {
		var t string
		t, temp = temp[len(temp) - 1], temp[:len(temp) - 1]
		result = append(result, t)
	}

	return result, nil
}

func removeLexisAndNot(stack []string) []string {
	n_stack := []string{}
	stack_len := len(stack) - 1
	for i, curr := range stack {
		if curr == "and" {
			if i + 1 <= stack_len && stack[i + 1] == "not" {
				continue
			} else {
				n_stack = append(n_stack, curr)
			}
		} else {
			n_stack = append(n_stack, curr)
		}
	}
	return n_stack
} 

func checkKeywordArrangement(stack []string, insert bool) ([]string, error) {
	stack_len := len(stack) - 1
	insert_pos := make([]int, 0)

	for i, curr := range stack {
		if !isOperator(curr) && curr != "(" && curr != ")" {
			if i + 1 <= stack_len && 
			!isOperator(stack[i + 1]) &&
			stack[i + 1] != "(" &&
			stack[i + 1]  != ")" {
				if insert {
					insert_pos = append(insert_pos, i)
				} else {
					return []string{}, errors.New("Missing operator between keywords where quotation marks required to denote phrase.")	

				}
			}
		}
	}

	for i := range insert_pos {
		stack = append(stack, "")
		copy(stack[i + 1:], stack[i:])
		stack[i] = "or"
	}

	return stack, nil
}
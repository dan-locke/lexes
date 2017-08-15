package parser

import (
	"strconv"
	"strings"
)

// Text taken from Lexiswebsite 
// ```Connectors operate in the following order of priority:
// - OR
// - W/n, PRE/n, NOT W/n
// - W/sent
// - W/para
// - W/SEG
// - NOT W/SEG
// - AND
// - AND NOT
// If you use two or more of the same connector, they operate left to right. 
// If the "n" (number) connectors have different numbers, the smallest number is operated on first. 
// You cannot use the W/para and W/sent connectors with a proximity connector (e.g., W/n).```
func isNum(word string) bool {
	_, err := strconv.Atoi(word)
	if err == nil {
		return true
	} else {
		return false
	}
}

func isOperator(word string) bool {
	return word == "or" || 
		word == "and" ||
		word == "not" ||
		word == "w/p" ||
		word == "w/s" ||
		(strings.HasPrefix(word, "w/") && isNum(word[2:])) 	
}

func checkProximityOperators(word string) int {
	ret := 0
	if strings.HasPrefix(word, "w/") && word != "w/p" && word != "w/s" {
		if isNum(word[2:]) {
			ret = 1
		} else {
			ret = 2
		}
	} 
	return ret
}
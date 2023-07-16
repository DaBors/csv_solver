package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"unicode"
	"github.com/shopspring/decimal"
)

type Cell struct {
	value     string
	equation  string
	column    int
	row       int
	evaluated bool
}

var table [][]Cell

func main() {
	readTableFromFile("./transactions.csv")
	evaluateTable()
	for i := range table {
		for j := range table[i] {
			fmt.Print(table[i][j].value + "|")
		}
		fmt.Println()
	}
}

const (
	// Equation types with their prefixes and parentheses
	CONCAT       = "concat"
	TEXT         = "text"
	SUM          = "sum"
	FORMULA_COPY = "^^"
	SPREAD       = "spread"
	INC_FROM     = "incFrom"
	SPLIT        = "split"
)

// Gets the column index for the given letter
//
// @param letter string
// @return int
func getColumnIndex(letter string) int {
	return int(letter[0] - 'A')
}

// Evaluates the given concat equation
//
// @param parameters []string
// @return string
func evaluateConcat(parameters []string) string {
	result := ""
	for _, parameter := range parameters {
		if parameter == "#ERROR" {
			return "#ERROR"
		} else if !strings.HasPrefix(parameter, "\"") || !strings.HasSuffix(parameter, "\"") {
			return "#CANNOT_CONCATENATE_NON_STRING"
		}
		parameter = parameter[1 : len(parameter)-1]
		result += parameter
	}
	result = strings.Replace(result, "\"", "", -1)
	return "\"" + result + "\""
}

// Evaluates the given split equation
//
// @param parameters []string
// @return []string
func evaluateSplit(parameters []string) string {
	for _, parameter := range parameters {
		if parameter == "#ERROR" {
			return "#ERROR"
		}
	}
	// TODO: Handle the case where the split parameters are strings with " and not numbers
	return strings.Join(parameters[:len(parameters)-2], ",")
}

// Evaluates the given sum equation
//
// @param parameters []string
// @return string
func evaluateSum(parameters []string) string {
	result := decimal.Zero
	for _, parameter := range parameters {
		if parameter == "#ERROR" {
			return "#ERROR"
		}
		decimalParameter, _ := decimal.NewFromString(parameter)
		result = result.Add(decimalParameter)
	}
	return result.String()
}

func evaluateEquation(equation string, row int, column int) ([]string, string) {
	if table[row][column].evaluated || table[row][column].equation == "" {
		return []string{table[row][column].value}, ""
	}

	stack := ""
	parentheses := 0
	quote := false
	results := []string{}

	for i := 0; i < len(equation); i++ {
		stack += string(equation[i])
		if equation[i] == '(' && !quote {
			parentheses++
		} else if equation[i] == ')' && !quote {
			parentheses--
			if parentheses == 0 {
				evaluationResults, err := evaluateEquation(stack[strings.Index(stack, "(")+1:len(stack)-1], row, column)
				if err == "#ERROR" {
					return make([]string, 0), "#ERROR"
				}
				stack = stack[:strings.Index(stack, "(")]
				// TODO: evaluate the equation with the potential functions
				if strings.HasSuffix(stack, CONCAT) {
					stack = stack[:len(stack)-len(CONCAT)]
					stack += evaluateConcat(evaluationResults)
				} else if strings.HasSuffix(stack, TEXT) {
					stack = stack[:len(stack)-len(TEXT)]
					stack += "\"" + evaluationResults[0] + "\""
				} else if strings.HasSuffix(stack, INC_FROM) {
					stack = stack[:len(stack)-len(INC_FROM)]
					stack += evaluationResults[0]
				} else if strings.HasSuffix(stack, SUM) {
					stack = stack[:len(stack)-len(SUM)]
					stack += evaluateSum(evaluationResults)
				} else if strings.HasSuffix(stack, SPREAD) {
					stack = stack[:len(stack)-len(SPREAD)]
					stack += strings.Join(evaluationResults, ",")
				} else if strings.HasSuffix(stack, SPLIT) {
					stack = stack[:len(stack)-len(SPLIT)]
					stack += evaluateSplit(evaluationResults)
				} 
			} else if parentheses < 0 {
				return make([]string, 0), "#ERROR"
			}
		} else if equation[i] == '"' {
			quote = !quote
			continue
		} else if equation[i] == ' ' && !quote {
			stack = stack[:len(stack)-1]
			continue
		} else if parentheses == 0 && equation[i] == ',' && !quote {
			results = append(results, stack[:len(stack)-1])
			stack = ""
			quote = false
			continue
		} else if unicode.IsDigit(rune(equation[i])) && !quote && len(stack) > 1 {
			if unicode.IsLetter(rune(equation[i-1])) {
				// TODO: Handle row numbers with more than one digit
				referenceRow, _ := strconv.Atoi(string(equation[i] - 1))
				referenceColumn := getColumnIndex(string(equation[i-1]))
				stack = stack[:len(stack)-2]
				stack += strings.Join(evaluateEquation(table[referenceRow][referenceColumn].equation, referenceRow, referenceColumn))
			}
		} else if strings.HasSuffix(stack, FORMULA_COPY) {
			stack = stack[:len(stack)-len(FORMULA_COPY)]
			stack += strings.Join(evaluateEquation(table[row-1][column].equation, row-1, column))
		}
	}

	if len(stack) > 0 {
		for i := 0; i < len(stack); i++ {
			if parentheses == 0 && stack[i] == ',' && !quote {
				results = append(results, stack[:i])
				stack = stack[i+1:]
				i = 0
				quote = false
				continue
			}
		}
	}

	if len(results) == 0 && stack != "" {
		results = append(results, stack)
	} else if stack != "" {
		results = append(results, stack)
	} else if len(results) == 0 {
		results = append(results, "")
	}

	return results, ""

}

// Evaluates the table
//
// @param table [][]cell
func evaluateTable() {
	for i := range table {
		for j := range table[i] {
			if table[i][j].equation != "" {
				value, err := evaluateEquation(table[i][j].equation, i, j)
				table[i][j].evaluated = true
				if err == "#ERROR" {
					table[i][j].value = "#ERROR"
					continue
				}

				table[i][j].value = value[0]
			}
			table[i][j].evaluated = true
		}
	}
}

// Reads a table from a file
//
// @param filePath string
func readTableFromFile(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		row := []Cell{}

		for rowNumber, cellText := range strings.Split(scanner.Text(), "|") {
			//handle equation texts starting with =
			if strings.HasPrefix(cellText, "=") {
				row = append(row, Cell{equation: cellText[1:], column: rowNumber, row: lineNumber - 1})
				continue
			}
			row = append(row, Cell{value: cellText, column: rowNumber, row: lineNumber - 1})
		}

		table = append(table, row)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

}

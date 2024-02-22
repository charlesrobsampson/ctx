package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charlesrobsampson/ctxclient"
)

var (
	HOST      = os.Getenv("CTX_HOST")
	USER      = os.Getenv("CTX_USER")
	timeUnits = map[string]string{
		"s": "seconds",
		"m": "minutes",
		"h": "hours",
		"d": "days",
		"w": "weeks",
		"M": "months",
		"y": "years",
	}
)

func main() {
	allArgs := os.Args
	args := allArgs[1:]
	// fmt.Printf("args: %v\n", args)
	output := ""
	ctxClient := ctxclient.NewContextClient(HOST, USER)
	if len(args) == 0 {
		// c, err := ctxClient.GetContext("context")
		c, err := ctxClient.GetCurrentContext()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		output, err = stringifyContext(c)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		if c.UserId == "" {
			output = "no current context"
		} else {
			if c.ParentId != "" {
				parentContext, err := ctxClient.GetContext(c.ParentId)
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("parent: %s\n", parentContext.Name)
				fmt.Printf("parentId: %s\n", parentContext.ContextId)
			}
			currentTime := time.Now().UTC()
			startedTime, err := time.Parse(ctxclient.SkDateFormat, c.Created)
			if err != nil {
				fmt.Printf("Error parsing start time: %v\n", err)
				os.Exit(1)
			}
			diff := currentTime.Sub(startedTime).Minutes()
			fmt.Printf("minutes on current context: %d\n", int(diff+0.5))
		}
	} else {
		cmd := args[0]
		args = args[1:]
		switch cmd {
		case "get":
			if len(args) == 0 {
				fmt.Printf("Error: missing contextId\n")
				os.Exit(1)
			}
			contextId := args[0]
			c, err := ctxClient.GetContext(contextId)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			output, err = stringifyContext(c)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			if output == "{}" {
				output = fmt.Sprintf("context '%s' not found", contextId)
			}
		case "last":
			c, err := ctxClient.GetContext("last")
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			output, err = stringifyContext(c)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			if output == "{}" {
				output = "no last context"
			}
		case "list":
			unit := getLine(fmt.Sprintf("which time unit would you like to query by (default h)?\noptions: (%s)\n", displayUnits(timeUnits)), false)
			if unit == "" {
				unit = "h"
			}
			start := getLine(fmt.Sprintf("how many %s back would you like to start your query (default 1)? ", timeUnits[unit]), false)
			end := getLine(fmt.Sprintf("how many %s back would you like to end your query (default 0)? ", timeUnits[unit]), false)
			c, err := ctxClient.ListContexts(ctxclient.QSParams{
				"start": start,
				"end":   end,
				"unit":  unit,
			})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			output, err = stringifyList(c)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			if output == "{}" {
				output = "no last context"
			}
		case "summary":
			fmt.Println("WARNING! This is still in development and doesn't really work yet")
			unit := getLine(fmt.Sprintf("which time unit would you like to query by (default h)?\noptions: (%s)\n", displayUnits(timeUnits)), false)
			if unit == "" {
				unit = "h"
			}
			start := getLine(fmt.Sprintf("how many %s back would you like to start your query (default 1)? ", timeUnits[unit]), false)
			end := getLine(fmt.Sprintf("how many %s back would you like to end your query (default 0)? ", timeUnits[unit]), false)
			_, err := ctxClient.ListFormattedContexts(ctxclient.QSParams{
				"start": start,
				"end":   end,
				"unit":  unit,
			})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			// output, err = stringifyList(c)
			// if err != nil {
			// 	fmt.Printf("Error: %v\n", err)
			// 	os.Exit(1)
			// }
			// if output == "{}" {
			// 	output = "no last context"
			// }
		case "switch":
			isSubContext := false
			sameParent := false
			if len(args) > 0 {
				isSubContext = args[0] == "sub"
				sameParent = args[0] == "same"
			}
			currentContext, err := ctxClient.GetCurrentContext()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			if currentContext.Name != "" {
				if currentContext.ParentId != "" {
					parentContext, err := ctxClient.GetContext(currentContext.ParentId)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						os.Exit(1)
					}
					fmt.Printf("parent: %s\n", parentContext.Name)
					fmt.Printf("parentId: %s\n", parentContext.ContextId)
				}
				fmt.Printf("current: %s\n", currentContext.Name)
			}
			c := ctxclient.Context{}
			name := getLine("new context name: ", true)
			c.Name = name
			addNotes(&c, "Enter notes for this context (endline with \\ for multiline): ")

			if isSubContext {
				c.ParentId = currentContext.ContextId
			} else if sameParent {
				c.ParentId = currentContext.ParentId
			} else {
				parentId := getLine("parentId [optional]: ", false)
				if len(parentId) > 0 {
					c.ParentId = parentId
				}
			}
			// fmt.Printf("new context: %+v\n", c)
			output, err = stringifyContext(&c)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("new context:\n%s\n", output)
			makeSwitch := confirm("make switch? [Y/n]: ", "y")
			if makeSwitch {
				newContextId, err := ctxClient.UpdateContext(&c)
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					os.Exit(1)
				}
				output = fmt.Sprintf("\nupdated context: %s\nwith contextId: %s\n", c.Name, newContextId)
			} else {
				output = "cancelled"
			}
		case "note":
			c, err := ctxClient.GetCurrentContext()
			c.ContextId = ""
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			addNotes(c, "add note (endline with \\ for multiline): ")
			_, err = stringifyContext(c)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			newContextId, err := ctxClient.UpdateContext(c)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			output = fmt.Sprintf("added note to '%s'\nwith contextId: %s\n", c.Name, newContextId)
		case "close":
			contextId := "current"
			if len(args) > 0 {
				contextId = args[0]
			}
			contextId = getLastHash(contextId)
			response, err := ctxClient.CloseContext(contextId)
			if err != nil && response != "no current context" && response != fmt.Sprintf("context 'context#%s' not found", contextId) {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			output = response
		case "h":
			fmt.Printf("HOST: %s\n", HOST)
			fmt.Printf("USER: %s\n", USER)
			output = "history? what should I call this? last? I want all contexts from now-x units to now+y units"
		default:
			output = "Unknown command"
		}
	}
	println(output)
}

func stringifyContext(c *ctxclient.Context) (string, error) {
	ctxJson, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return "", err
	}
	return string(ctxJson), nil
}

func stringifyList(c *[]ctxclient.Context) (string, error) {
	ctxJson, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return "", err
	}
	return string(ctxJson), nil
}

func getLine(prompt string, isRequired bool) string {
	fmt.Print(prompt)
	scn := bufio.NewScanner(os.Stdin)
	scn.Scan()
	txt := scn.Text()
	txt = strings.TrimSpace(txt)
	if len(txt) == 0 && isRequired {
		fmt.Println("Error: this is required")
		return getLine(prompt, isRequired)
	}
	return txt
}

func getMultiLine(prompt string) []string {
	scn := bufio.NewScanner(os.Stdin)
	var lines []string
	fmt.Println(prompt)
	for scn.Scan() {
		line := scn.Text()
		if len(line) == 0 {
			break
		}
		// var r byte = line[len(line)-1]
		// fmt.Printf("byte: %v\n", r)
		if line[len(line)-1] != '\\' {
			if len(line) > 0 {
				line = strings.TrimSpace(line)
				lines = append(lines, line)
			}
			break
		} else {
			line = line[:len(line)-1]
			line = strings.TrimSpace(line)
			lines = append(lines, line)
		}
	}

	if err := scn.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		// break
	}
	return lines
}

func isNullJSON(m json.RawMessage) bool {
	return len(m) == 0 || string(m) == "null"
}

func confirm(prompt string, def string) bool {
	txt := getLine(prompt, false)
	if txt == "" {
		txt = def
	}
	txt = strings.ToLower(txt)
	if txt == "y" || txt == "yes" {
		return true
	}
	return false
}

func addNotes(c *ctxclient.Context, prompt string) []byte {
	notes := getMultiLine(prompt)
	notesJSON, err := json.Marshal(notes)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if !isNullJSON(notesJSON) {
		c.Notes = notesJSON
	}
	return notesJSON
}

func getLastHash(sk string) string {
	if strings.Contains(sk, "#") {
		ctxTimestamp := strings.Split(sk, "#")
		sk = ctxTimestamp[len(ctxTimestamp)-1]
	}
	return sk
}

func displayUnits(units map[string]string) string {
	list := []string{}
	for k, v := range units {
		list = append(list, fmt.Sprintf("%s [%s]", k, v))
	}
	return strings.Join(list, " | ")
}

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/charlesrobsampson/ctxclient"
)

var (
	HOST        = os.Getenv("CTX_HOST")
	USER        = os.Getenv("CTX_USER")
	EXPORT_TYPE = defaultEnv("CTX_EXPORT_TYPE", "json")
	timeUnits   = map[string]string{
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
	if HOST == "" {
		fmt.Println("CTX_HOST environment variable not set")
		os.Exit(1)
	}
	if USER == "" {
		fmt.Println("CTX_USER environment variable not set")
		os.Exit(1)
	}
	allArgs := os.Args
	args := allArgs[1:]
	// fmt.Printf("args: %v\n", args)
	output := ""
	ctxClient := ctxclient.NewContextClient(HOST, USER)
	qClient := ctxclient.NewQueueClient(HOST, USER)
	if len(args) == 0 {
		output = currentCtx(ctxClient)
	} else {
		cmd := args[0]
		args = args[1:]
		switch cmd {
		case "g", "get":
			output = getCtx(ctxClient, args)
		case "l", "last":
			output = lastCtx(ctxClient)
		case "ls", "list":
			output = listCtx(ctxClient)
		case "sum", "summary":
			output = summaryCtx(ctxClient)
		case "s", "switch":
			output = switchCtx(ctxClient, args)
		case "-", "sub":
			output = switchCtx(ctxClient, []string{"sub"})
		case "=", "same":
			output = switchCtx(ctxClient, []string{"same"})
		case "n", "note":
			output = addNoteCtx(ctxClient)
		case "c", "close":
			output = closeCtx(ctxClient, args)
		case "r", "resume":
			output = resumeCtx(ctxClient, args)
		case "q":
			if len(args) == 0 {
				output = listQueue(qClient)
			} else {
				cmd := args[0]
				switch cmd {
				case "g", "get":
					output = getQueue(qClient, args)
				case "a", "add":
					output = addQueue(qClient)
				case "d", "do":
					output = doQueue(qClient, ctxClient, args)
				case "n", "note":
					output = addNoteQueue(qClient, args)
				case "c", "close":
					output = closeQueue(qClient, args)
				default:
					output = "unkown q command"
				}
			}
		default:
			output = "Unknown command"
		}
	}
	println(output)
}

func currentCtx(ctxClient *ctxclient.ContextClient) string {
	output := ""
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
	return output
}

func getCtx(ctxClient *ctxclient.ContextClient, args []string) string {
	output := ""
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
	return output
}

func lastCtx(ctxClient *ctxclient.ContextClient) string {
	output := ""
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
	return output
}

func listCtx(ctxClient *ctxclient.ContextClient) string {
	output := ""
	start, end, unit := getQueryWindow()
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
	return output
}

func summaryCtx(ctxClient *ctxclient.ContextClient) string {
	output := ""
	start, end, unit := getQueryWindow()
	ctxs, err := ctxClient.ListFormattedContexts(ctxclient.QSParams{
		"start": start,
		"end":   end,
		"unit":  unit,
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	output, err = stringifyFormatted(&ctxs)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
	return output
}

func switchCtx(ctxClient *ctxclient.ContextClient, args []string) string {
	output := ""
	isSubContext := false
	sameParent := false
	if len(args) > 0 {
		isSubContext = args[0] == "sub" || args[0] == "-"
		sameParent = args[0] == "same" || args[0] == "="
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
	addNotes(&c, "Enter notes for this context (endline with \\ for multiline): ")
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
	return output
}

func addNoteCtx(ctxClient *ctxclient.ContextClient) string {
	output := ""
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
	return output
}

func closeCtx(ctxClient *ctxclient.ContextClient, args []string) string {
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
	return response
}

func resumeCtx(ctxClient *ctxclient.ContextClient, args []string) string {
	output := ""
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
	c.ContextId = ""
	output, err = stringifyContext(c)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("resuming context:\n%s\n", output)
	addNotes(c, "Add notes to this context (endline with \\ for multiline): ")
	output, err = stringifyContext(c)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("new context:\n%s\n", output)
	makeSwitch := confirm("make switch? [Y/n]: ", "y")
	if makeSwitch {
		newContextId, err := ctxClient.UpdateContext(c)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		output = fmt.Sprintf("\nupdated context: %s\nwith contextId: %s\n", c.Name, newContextId)
	} else {
		output = "cancelled"
	}
	return output
}

func listQueue(qClient *ctxclient.QueueClient) string {
	output := ""
	fmt.Println("queue:")
	q, err := qClient.ListQueue()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	output, err = stringifyQueueList(q)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	return output
}

func getQueue(qClient *ctxclient.QueueClient, args []string) string {
	output := ""
	args = args[1:]
	if len(args) == 0 {
		fmt.Printf("Error: missing queueId\n")
		os.Exit(1)
	}
	qId := args[0]
	q, err := qClient.GetQueue(qId)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	output, err = stringifyQueue(q)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if output == "{}" {
		output = fmt.Sprintf("queue '%s' not found", qId)
	}
	return output
}

func addQueue(qClient *ctxclient.QueueClient) string {
	output := ""
	fmt.Println("add to queue")
	q := ctxclient.Queue{}
	name := getLine("new queue name: ", true)
	q.Name = name
	addQueueNotes(&q, "Enter notes for this queue (endline with \\ for multiline): ")
	addQueue := confirm("add to queue? [Y/n]: ", "y")
	if addQueue {
		newQueueId, err := qClient.UpdateQueue(&q)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		output = fmt.Sprintf("\nadded queue: %s\nwith id: %s\n", q.Name, newQueueId)
	} else {
		output = "cancelled"
	}
	return output
}

func doQueue(qClient *ctxclient.QueueClient, ctxClient *ctxclient.ContextClient, args []string) string {
	output := ""
	c := ctxclient.Context{}
	args = args[1:]
	if len(args) == 0 {
		fmt.Printf("Error: missing queueId\n")
		os.Exit(1)
	}
	qId := args[0]
	q, err := qClient.GetQueue(qId)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if q.Started != "" {
		fmt.Printf("queue '%s' already started\n", q.Name)
		if q.ContextId != "" {
			fmt.Printf("by context: %s\n", q.ContextId)
		}
		os.Exit(0)
	}
	fmt.Printf("starting queue '%s'\nname: %s\n", qId, q.Name)

	c.Name = fmt.Sprintf("%s | queue", q.Name)
	parentId := getLine("parentId [optional]: ", false)
	if len(parentId) > 0 {
		c.ParentId = parentId
	}
	if !isNullJSON(q.Notes) {
		previous := []string{}
		err := json.Unmarshal([]byte(q.Notes), &previous)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		qNoteString, err := jsonMarshalIndent(q.Notes, false)
		// qNoteString, err := json.MarshalIndent(q.Notes, "", "  ")
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("notes from queue:\n%s\n", string(qNoteString))
		combineNotes(&c, previous, "add note (endline with \\ for multiline): ")
	} else {
		addNotes(&c, "Enter notes for this context (endline with \\ for multiline): ")
	}
	_, err = stringifyContext(&c)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
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
		_, err = qClient.StartQueue(qId, newContextId)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		output = fmt.Sprintf("\nupdated context: %s\nwith contextId: %s\n", c.Name, newContextId)
	} else {
		output = "cancelled"
	}
	return output
}

func addNoteQueue(qClient *ctxclient.QueueClient, args []string) string {
	output := ""
	args = args[1:]
	if len(args) == 0 {
		fmt.Printf("Error: missing queueId\n")
		os.Exit(1)
	}
	qId := args[0]
	q, err := qClient.GetQueue(qId)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	addQueueNotes(q, "add note (endline with \\ for multiline): ")
	_, err = stringifyQueue(q)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	newQueueId, err := qClient.UpdateQueue(q)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	output = fmt.Sprintf("added note to '%s'\nwith queueId: %s\n", q.Name, newQueueId)
	return output
}

func closeQueue(qClient *ctxclient.QueueClient, args []string) string {
	output := ""
	args = args[1:]
	if len(args) == 0 {
		fmt.Printf("Error: missing queueId\n")
		os.Exit(1)
	}
	qId := args[0]
	_, err := qClient.StartQueue(qId, "")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	output = fmt.Sprintf("queue %s closed", qId)
	return output
}

func getQueryWindow() (string, string, string) {
	unit := getLine(fmt.Sprintf("which time unit would you like to query by (default h)?\noptions: (%s)\n", displayUnits(timeUnits)), false)
	if unit == "" {
		unit = "h"
	}
	start := getLine(fmt.Sprintf("how many %s back would you like to start your query (default 1)? ", timeUnits[unit]), false)
	end := getLine(fmt.Sprintf("how many %s back would you like to end your query (default 0)? ", timeUnits[unit]), false)
	return start, end, unit
}

func stringifyContext(c *ctxclient.Context) (string, error) {
	if isNullJSON(c.Notes) {
		c.Notes = []byte{}
	}
	ctxJson, err := jsonMarshalIndent(c, false)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return "", err
	}
	if EXPORT_TYPE == "yaml" {
		return printYaml(ctxJson)
	}
	// ctxJson := buffer.Bytes()
	return string(ctxJson), nil
}

func stringifyQueue(q *ctxclient.Queue) (string, error) {
	qJson, err := jsonMarshalIndent(q, false)
	// qJson, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return "", err
	}
	if EXPORT_TYPE == "yaml" {
		return printYaml(qJson)
	}
	return string(qJson), nil
}

func stringifyList(c *[]ctxclient.Context) (string, error) {
	ctxJson, err := jsonMarshalIndent(c, false)
	// ctxJson, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return "", err
	}
	if EXPORT_TYPE == "yaml" {
		return printYaml(ctxJson)
	}
	return string(ctxJson), nil
}

func stringifyFormatted(c *[]ctxclient.FormattedContext) (string, error) {
	ctxJson, err := jsonMarshalIndent(c, false)
	// ctxJson, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return "", err
	}
	if EXPORT_TYPE == "yaml" {
		return printYaml(ctxJson)
	}
	return string(ctxJson), nil
}

func stringifyQueueList(q *[]ctxclient.Queue) (string, error) {
	qJson, err := jsonMarshalIndent(q, false)
	// qJson, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return "", err
	}
	if EXPORT_TYPE == "yaml" {
		return printYaml(qJson)
	}
	return string(qJson), nil
}

func printYaml(data []byte) (string, error) {
	var jsonInterface interface{}
	err := json.Unmarshal(data, &jsonInterface)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return "", err
	}
	yamlBytes, err := yaml.Marshal(jsonInterface)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return "", err
	}
	return string(yamlBytes), nil
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
	notesJSON, err := jsonMarshal(notes, false)
	// notesJSON, err := json.Marshal(notes)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if !isNullJSON(notesJSON) {
		c.Notes = notesJSON
	}
	return notesJSON
}

func combineNotes(c *ctxclient.Context, previous []string, prompt string) []byte {
	notes := getMultiLine(prompt)
	previous = append(previous, notes...)
	notesJSON, err := jsonMarshal(previous, false)
	// notesJSON, err := json.Marshal(previous)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if !isNullJSON(notesJSON) {
		c.Notes = notesJSON
	}
	return notesJSON
}

func addQueueNotes(q *ctxclient.Queue, prompt string) []byte {
	notes := getMultiLine(prompt)
	notesJSON, err := jsonMarshal(notes, false)
	// notesJSON, err := json.Marshal(notes)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if !isNullJSON(notesJSON) {
		q.Notes = notesJSON
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
func jsonMarshal(data interface{}, escapeHTML bool) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(escapeHTML)
	err := encoder.Encode(data)
	return buffer.Bytes(), err
}

func jsonMarshalIndent(data interface{}, escapeHTML bool) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(escapeHTML)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(data)
	return buffer.Bytes(), err
}

func defaultEnv(envName, defaultValue string) string {
	val := os.Getenv(envName)
	if val == "" {
		val = defaultValue
	}
	return val
}

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/charlesrobsampson/ctxclient"
)

const version = "v1.3.1"

var (
	HOST                = os.Getenv("CTX_HOST")
	USER                = os.Getenv("CTX_USER")
	CTX_DOCS_PATH       = os.Getenv("CTX_DOCS_PATH")
	CTX_GITHUB_DOCS_URL = os.Getenv("CTX_GITHUB_DOCS_URL")
	EXPORT_TYPE         = defaultEnv("CTX_EXPORT_TYPE", "json")
	CTX_REPORT_UPDATES  = defaultEnv("CTX_REPORT_UPDATES", "true")
	CTX_DEFAULT_EDITOR  = defaultEnv("CTX_DEFAULT_EDITOR", "code")
	timeUnits           = map[string]string{
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
	output := ""
	ctxClient := ctxclient.NewContextClient(HOST, USER)
	qClient := ctxclient.NewQueueClient(HOST, USER)
	current, err := ctxClient.GetCurrentContext()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if len(args) == 0 {
		output = currentCtx(ctxClient, current)
	} else {
		cmd := args[0]
		args = args[1:]
		switch cmd {
		case "v", "version":
			output = checkVersions(ctxClient, true)
		case "g", "get":
			outputChan := make(chan string)
			versionCheckChan := make(chan string)
			go func(outputChan chan string) {
				output := getCtx(ctxClient, args)
				outputChan <- output
			}(outputChan)
			go func(versionCheckChan chan string) {
				versionCheck := checkVersions(ctxClient, false)
				versionCheckChan <- versionCheck
			}(versionCheckChan)
			output = <-outputChan
			versionCheck := <-versionCheckChan
			if len(versionCheck) > 0 {
				output += versionCheck
			}
		case "l", "last":
			output = lastCtx(ctxClient)
		case "ls", "list":
			output = listCtx(ctxClient)
		case "sum", "summary":
			output = summaryCtx(ctxClient)
		case "s", "switch":
			output = switchCtx(ctxClient, current, args)
		case "-", "sub":
			output = switchCtx(ctxClient, current, []string{"sub"})
		case "=", "same":
			output = switchCtx(ctxClient, current, []string{"same"})
		case "n", "note":
			outputChan := make(chan string)
			versionCheckChan := make(chan string)
			go func(outputChan chan string) {
				output := addNoteCtx(ctxClient, current)
				outputChan <- output
			}(outputChan)
			go func(versionCheckChan chan string) {
				versionCheck := checkVersions(ctxClient, false)
				versionCheckChan <- versionCheck
			}(versionCheckChan)
			output = <-outputChan
			versionCheck := <-versionCheckChan
			if len(versionCheck) > 0 {
				output += versionCheck
			}
		case "c", "close":
			outputChan := make(chan string)
			versionCheckChan := make(chan string)
			go func(outputChan chan string) {
				output := closeCtx(ctxClient, args)
				outputChan <- output
			}(outputChan)
			go func(versionCheckChan chan string) {
				versionCheck := checkVersions(ctxClient, false)
				versionCheckChan <- versionCheck
			}(versionCheckChan)
			output = <-outputChan
			versionCheck := <-versionCheckChan
			if len(versionCheck) > 0 {
				output += versionCheck
			}
		case "r", "resume":
			output = resumeCtx(ctxClient, args)
		case "tm", "timeMachine":
			if current.LastContext == "" {
				output = fmt.Sprintf("current context '%s' has no last context\n", current.Name)
			} else {
				timeMachineCtx(ctxClient, current.LastContext)
			}
		case "p", "parents":
			if len(args) > 0 {
				parentsCtx(ctxClient, args[0])
			} else {
				if current.ParentId == "" {
					output = fmt.Sprintf("current context '%s' has no parent\n", current.Name)
				} else {
					parentsCtx(ctxClient, current.ParentId)
				}
			}
		case "q":
			if len(args) == 0 {
				output = listQueue(qClient)
			} else {
				cmd := args[0]
				switch cmd {
				case "g", "get":
					outputChan := make(chan string)
					versionCheckChan := make(chan string)
					go func(outputChan chan string) {
						output := getQueue(qClient, args)
						outputChan <- output
					}(outputChan)
					go func(versionCheckChan chan string) {
						versionCheck := checkVersions(ctxClient, false)
						versionCheckChan <- versionCheck
					}(versionCheckChan)
					output = <-outputChan
					versionCheck := <-versionCheckChan
					if len(versionCheck) > 0 {
						output += versionCheck
					}
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
		case "d", "doc":
			if CTX_DOCS_PATH == "" {
				output = "CTX_DOCS_PATH environment variable not set"
			} else {
				cmd := "e"
				if len(args) > 0 {
					cmd = args[0]
					args = args[1:]
				}
				dateString := strings.Split(current.ContextId, "T")[0]
				dateSlice := strings.Split(dateString, "-")
				year := dateSlice[0]
				month := dateSlice[1]
				day := dateSlice[2]
				dirPath := fmt.Sprintf("ctx/%s/%s/%s", year, month, day)
				fileName := fmt.Sprintf("%s.md", strings.ReplaceAll(current.Name, " ", "_"))
				absolutePath := fmt.Sprintf("%s/%s/%s", CTX_DOCS_PATH, dirPath, fileName)

				switch cmd {
				case "e", "edit":
					if current.Document.RealtivePath == "" {
						// create new doc
						fmt.Println("creating new doc")
						// first check if file exists
						_, err := os.Stat(absolutePath)
						if err != nil {
							mkdirCmd := exec.Command("mkdir", "-p", fmt.Sprintf("%s/%s", CTX_DOCS_PATH, dirPath))
							err := mkdirCmd.Run()
							if err != nil {
								fmt.Printf("Error: %v\n", err)
								os.Exit(1)
							}
							createCmd := exec.Command("touch", absolutePath)
							err = createCmd.Run()
							if err != nil {
								fmt.Printf("Error: %v\n", err)
								os.Exit(1)
							}
							if current.ParentId != "" {
								parentDoc, err := findParentDoc(ctxClient, current.ParentId)
								if err != nil {
									fmt.Printf("Error: %v\n", err)
									os.Exit(1)
								}
								if parentDoc.RealtivePath != "" {
									parentString := fmt.Sprintf("[parent doc](%s)\n", parentDoc.RealtivePath)
									os.WriteFile(absolutePath, []byte(parentString), 0644)
								}
							}
						}
						current.Document.RealtivePath = fmt.Sprintf("%s/%s", dirPath, fileName)
						if CTX_GITHUB_DOCS_URL != "" {
							current.Document.Github = fmt.Sprintf("%s/%s", CTX_GITHUB_DOCS_URL, current.Document.RealtivePath)
						}
						_, err = ctxClient.UpdateContext(current)
						if err != nil {
							fmt.Printf("Error adding doc to context: %v\n", err)
							os.Exit(1)
						}
					}
					// edit existing doc
					output = fmt.Sprintf("opening doc:\n%s", absolutePath)
					openCmd := exec.Command(CTX_DEFAULT_EDITOR, fmt.Sprintf("%s/%s", CTX_DOCS_PATH, current.Document.RealtivePath))
					err = openCmd.Run()
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						os.Exit(1)
					}
				case "l", "link":
					output = "not implemented"
					// link doc to context
					// if a doc is already linked, prompt to start new ctx
				case "o", "open":
					realtivePath := current.Document.RealtivePath
					// open doc in editor
					// if none, open closest parent
					// if no parent, return saying none found and prompt to create new
					if realtivePath == "" {
						parentDoc, err := findParentDoc(ctxClient, current.ContextId)
						if err != nil {
							fmt.Printf("Error: %v\n", err)
							os.Exit(1)
						}
						if parentDoc.RealtivePath != "" {
							realtivePath = parentDoc.RealtivePath
						}
					}
					if realtivePath == "" {
						output = "no related docs found. create one with\nctx doc edit"
					} else {
						output = fmt.Sprintf("opening doc: %s", realtivePath)
						openCmd := exec.Command(CTX_DEFAULT_EDITOR, fmt.Sprintf("%s/%s", CTX_DOCS_PATH, realtivePath))
						err = openCmd.Run()
						if err != nil {
							fmt.Printf("Error: %v\n", err)
							os.Exit(1)
						}
					}
				default:
					output = "unkown doc command"
				}
			}

		default:
			output = "Unknown command"
		}
	}
	println(output)
}

func currentCtx(ctxClient *ctxclient.ContextClient, c *ctxclient.Context) string {
	output, err := stringifyContext(c)
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

func switchCtx(ctxClient *ctxclient.ContextClient, currentContext *ctxclient.Context, args []string) string {
	output := ""
	isSubContext := false
	sameParent := false
	if len(args) > 0 {
		isSubContext = args[0] == "sub" || args[0] == "-"
		sameParent = args[0] == "same" || args[0] == "="
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
	output, err := stringifyContext(&c)
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

func addNoteCtx(ctxClient *ctxclient.ContextClient, c *ctxclient.Context) string {
	output := ""
	c.ContextId = ""
	addNotes(c, "add note (endline with \\ for multiline): ")
	_, err := stringifyContext(c)
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

func timeMachineCtx(ctxClient *ctxclient.ContextClient, ctxID string) {
	c, err := ctxClient.GetContext(ctxID)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	output, err := stringifyContext(c)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(output)
	if c.LastContext != "" {
		cont := confirm("keep going back in time? [y/N]: ", "n")
		if cont {
			fmt.Println("")
			timeMachineCtx(ctxClient, c.LastContext)
		}
	} else {
		fmt.Println("no more history for this thread")
	}
}

func parentsCtx(ctxClient *ctxclient.ContextClient, ctxID string) {
	c, err := ctxClient.GetContext(ctxID)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	output, err := stringifyContext(c)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(output)
	if c.ParentId != "" {
		cont := confirm("get parent? [y/N]: ", "n")
		if cont {
			fmt.Println("")
			parentsCtx(ctxClient, c.ParentId)
		}
	} else {
		fmt.Println("no more parents for this thread")
	}
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

func checkVersions(ctxClient *ctxclient.ContextClient, reportVersion bool) string {
	if CTX_REPORT_UPDATES == "false" && !reportVersion {
		return ""
	}
	output := make(chan string)
	go func(output chan string) {
		ctxapiVersion, err := ctxClient.GetVersion()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		latestCtxapi := getLatestRelease("ctxapi")
		ctxapiOutput := compareVersions("ctxapi", ctxapiVersion, latestCtxapi)
		if len(ctxapiOutput) > 0 {
			ctxapiOutput += fmt.Sprintf("pull ctxapi version %s and run\n  npm run deploy-prod\n", latestCtxapi)
		} else if reportVersion {
			ctxapiOutput = fmt.Sprintf("\ncurrent ctxapi version %s is up to date\n", ctxapiVersion)
		}
		output <- ctxapiOutput
	}(output)

	go func(output chan string) {
		latestCtx := getLatestRelease("ctx")
		ctxOutput := compareVersions("ctx", version, latestCtx)
		if len(ctxOutput) > 0 {
			ctxOutput += fmt.Sprintf("install ctx version %s with\n  go install github.com/charlesrobsampson/ctx@%s\nor\n  go install github.com/charlesrobsampson/ctx@latest\n", latestCtx, latestCtx)
		} else if reportVersion {
			ctxOutput = fmt.Sprintf("\ncurrent ctx version %s is up to date\n", version)
		}
		output <- ctxOutput
	}(output)

	return <-output + <-output
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

func getLatestRelease(pkg string) string {
	gitUrl := "https://github.com/charlesrobsampson/" + pkg
	head, err := http.Head(gitUrl + "/releases/latest")
	latestUrl := head.Request.URL.String()
	if err != nil {
		panic(err)
	}

	latest := strings.Split(latestUrl, gitUrl+"/releases/tag/")[1]
	return latest
}

func compareVersions(pkg, current, latest string) string {
	splitLatest := strings.Split(latest, ".")
	splitCurrent := strings.Split(current, ".")

	output := ""
	if stripNonNumbers(splitLatest[0]) > stripNonNumbers(splitCurrent[0]) {
		output = fmt.Sprintf("major update available for %s, latest version: %s\n", pkg, latest)
	} else if stripNonNumbers(splitLatest[0]) == stripNonNumbers(splitCurrent[0]) &&
		stripNonNumbers(splitLatest[1]) > stripNonNumbers(splitCurrent[1]) {
		output = fmt.Sprintf("minor update available for %s, latest version: %s\n", pkg, latest)
	} else if stripNonNumbers(splitLatest[0]) == stripNonNumbers(splitCurrent[0]) &&
		stripNonNumbers(splitLatest[1]) == stripNonNumbers(splitCurrent[1]) &&
		stripNonNumbers(splitLatest[2]) > stripNonNumbers(splitCurrent[2]) {
		output = fmt.Sprintf("patch update available for %s, latest version: %s\n", pkg, latest)
	}
	return output
}

func stripNonNumbers(s string) float64 {
	re := regexp.MustCompile(`[^0-9]`)
	str := re.ReplaceAllString(s, "")
	if len(str) == 0 {
		return 0
	}
	output, err := strconv.ParseFloat(str, 64)
	if err != nil {
		panic(err)
	}
	return output
}

func findParentDoc(ctxClient *ctxclient.ContextClient, ctxId string) (ctxclient.Document, error) {
	doc := ctxclient.Document{}
	c, err := ctxClient.GetContext(ctxId)
	if err != nil {
		return doc, err
	}
	if c.Document.RealtivePath != "" {
		return c.Document, nil
	}
	if c.ParentId == "" {
		return doc, nil
	}
	return findParentDoc(ctxClient, c.ParentId)
}

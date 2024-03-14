# ctx tool
## overview
This is a tool I made to help myself keep track of what I'm doing across different contexts like projects, tasks, notes etc. It aims to be simple, fast and extensible.

I liked the idea of it being a cli since a terminal is always within reach but it could also be expanded to have a simple GUI or web interface down the line. The data is stored in a dynamo db to allow accessing it from anywhere.

## setup
In order to use this tool, you must first deploy the [ctxapi](https://github.com/charlesrobsampson/ctxapi) (or have access to one someone else deployed) and then setup the following environment variables in your bash_prfoile or zshrc or whatever:

`export CTX_HOST="whateverUrlAmazonGaveYou`
`export CTX_USER="honestlyWhateverYouWantButItShouldBeUniqueToTheCtxapiEnv`

You could also add the optional `CTX_EXPORT_TYPE` variable to specify the export format (currently only "yaml" and "json" are supported with json as the default)

This is a go tool so you'll need go installed. Then you can install it with:

`go install github.com/charlesrobsampson/ctx@latest`

Once it is installed, you should be able to run `ctx` from your terminal to start using it!

## usage
There are essentially two different modes for using ctx
- context
  - this is to help keep track of and switch between different contexts like projects, tasks, notes etc.
- queue
 - this is a queue for future contexts or essentially a todo list

### Some basic context commands:
- `ctx` - shows current context
- `ctx note` - appends a note to the current context
- `ctx last` - shows last context
- `ctx switch` - switch context
  - `ctx switch sub` - switch to a new context nested under the current context
  - `ctx switch same` - switch to a new context with the same parent as the current context
- `ctx resume <contextId>` - continue on a specific context
- `ctx get <contextId>` - get details of a specific context
- `ctx list` - list contexts (this will ask you to define a time window to query)
- `ctx summary` - get a summary of contexts in a given time window
- `ctx close` - close current context

### Some basic queue commands:
- `ctx q` - list all items in the queue (anything that has been added but not started/closed)
- `ctx q add` - add an item to the queue
- `ctx q do <queueId>` - start a queued item (this will become your current context)
- `ctx q get <queueId>` - get details of a queued item (this works for past queues too)
- `ctx q note <queueId>` - add a note to a queued item
- `ctx q close <queueId>` - close a queued item (this will remove it from the queue without changing current context)

***note most commands have a shorthand version, a few examples:
- switch -> s
- switch sub -> -
- switch same -> =
- list -> ls
- get -> g
- summary -> sum


## future development
I know this isn't the best way to implement a cli in go. This wasn't intended to be published so it's possible I'll update it in the future, but I'm not against anyone who wants to makes improvements/additions getting access to do so. Just hit me up.

I will add features and improvements as I see fit but I would also be happy to collaborate with anyone who wants specific features.

A few things I might add in the future are:
- `-help` flag for commands
- ability to filter in the list and summary commands
- search for contexts based off certain criteria
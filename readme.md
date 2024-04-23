# Discord Dumper

A script written in go to dump all the messages sent, received, or have access to within a discord account using an account's discord token.

## Features

* Dump every chat message using a pool of bots logged into various servers
* Pool in a bunch of bots
* Auto join servers linked in scraped messages to improve pool of messages (to be added)
# Instructions

1. Clone the repository: `git clone https://github.com/vulnlol/GoDiscordDumper`
2. Enter the repository: `cd GoDiscordDumper`
3. Run the script: `go run dumper.go`
   1. Add as many tokens as you would like to.
   2. then run the scraper using the menu options

## Warning
Using this is against Discord's TOS I believe so you shouldn't use it. And it probably can get you IP address flagged as well as any account you use to log the messages cause you are basically accessing hundreds of thousands of messages and any metadata attached to them.
## What is this based off of?
I wrote it myself, though a couple of weeks ago I saw a post in r/privacy about how there was somebody selling mass amounts of discord messages and I was like "I'm going to reverse engineer that". So it's been in the back of my head and I started to work on it yesterday and I got it in a mostly functional state.

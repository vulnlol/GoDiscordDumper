package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	_ "time"
)

var (
	httpClient *http.Client
	fileMutex  sync.Mutex
)

type AccountData map[string]interface{}

type InviteInfo struct {
	GuildID    string `json:"guild_id"`
	GuildName  string `json:"guild_name"`
	InviteLink string `json:"invite_link"`
}

func init() {
	// Create a custom HTTP transport with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        10,               // Maximum number of idle connections to keep open
		MaxIdleConnsPerHost: 2,                // Maximum number of idle connections per host
		IdleConnTimeout:     30 * time.Second, // Idle connection timeout
	}
	httpClient = &http.Client{
		Transport: transport,
	}
}

func getUserClient() *http.Client {
	return httpClient
}

type UserInfo struct {
	ID string `json:"id"`
}

type Guild struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Channel struct {
	ID string `json:"id"`
}

func getUserInfo(token string) (UserInfo, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://discord.com/api/v9/users/@me", nil)
	if err != nil {
		return UserInfo{}, err
	}
	req.Header.Set("Authorization", token)
	resp, err := client.Do(req)
	if err != nil {
		return UserInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return UserInfo{}, fmt.Errorf("failed to get user info, status code: %d", resp.StatusCode)
	}
	var userInfo UserInfo
	err = json.NewDecoder(resp.Body).Decode(&userInfo)
	if err != nil {
		return UserInfo{}, err
	}
	return userInfo, nil
}

func getGuilds(token string) ([]Guild, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://discord.com/api/v9/users/@me/guilds", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var guilds []Guild
	err = json.NewDecoder(resp.Body).Decode(&guilds)
	if err != nil {
		return nil, err
	}
	return guilds, nil
}

func getChannels(token, guildID string) ([]Channel, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://discord.com/api/v9/guilds/%s/channels", guildID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var channels []Channel
	err = json.NewDecoder(resp.Body).Decode(&channels)
	if err != nil {
		return nil, err
	}
	return channels, nil
}

func addToken(token string) {
	accountsData := make(map[string]interface{})
	if _, err := os.Stat("accounts.json"); err == nil {
		file, err := ioutil.ReadFile("accounts.json")
		if err != nil {
			fmt.Println("Error reading accounts.json:", err)
			return
		}
		err = json.Unmarshal(file, &accountsData)
		if err != nil {
			fmt.Println("Error unmarshalling accounts.json:", err)
			return
		}
	}

	userInfo, err := getUserInfo(token)
	if err != nil {
		fmt.Println("Error getting user info:", err)
		return
	}

	guilds, err := getGuilds(token)
	if err != nil {
		fmt.Println("Error getting guilds:", err)
		return
	}

	guildInfo := make(map[string]interface{})
	for _, guild := range guilds {
		channels, err := getChannels(token, guild.ID)
		if err != nil {
			fmt.Println("Error getting channels for guild", guild.Name, ":", err)
			continue
		}
		channelIDs := make([]string, len(channels))
		for i, channel := range channels {
			channelIDs[i] = channel.ID
		}
		guildInfo[guild.Name] = map[string]interface{}{
			"guild_id": guild.ID,
			"channels": channelIDs,
		}
	}
	accountsData[token] = map[string]interface{}{
		"user_id": userInfo.ID,
		"guilds":  guildInfo,
	}
	data, err := json.MarshalIndent(accountsData, "", "    ")
	if err != nil {
		fmt.Println("Error marshalling data:", err)
		return
	}
	err = ioutil.WriteFile("accounts.json", data, 0644)
	if err != nil {
		fmt.Println("Error writing to accounts.json:", err)
		return
	}
	fmt.Println("Token added successfully.")
}

func scrapeData() {
	accountsData := make(map[string]interface{})
	if _, err := os.Stat("accounts.json"); err == nil {
		file, err := ioutil.ReadFile("accounts.json")
		if err != nil {
			fmt.Println("Error reading accounts.json:", err)
			return
		}
		err = json.Unmarshal(file, &accountsData)
		if err != nil {
			fmt.Println("Error unmarshalling accounts.json:", err)
			return
		}
	} else {
		fmt.Println("No accounts found. Please add tokens first.")
		return
	}

	const batchSize = 420 // Define batch size

	for token := range accountsData {
		userInfo, err := getUserInfo(token)
		if err != nil {
			fmt.Println("Error getting user info:", err)
			continue
		}
		fmt.Println("User ID:", userInfo.ID)

		guilds, err := getGuilds(token)
		if err != nil {
			fmt.Println("Error getting guilds:", err)
			continue
		}

		for _, guild := range guilds {
			channels, err := getChannels(token, guild.ID)
			if err != nil {
				fmt.Println("Error getting channels for guild", guild.Name, ":", err)
				continue
			}
			var messageBuffer []map[string]interface{}
			for _, channel := range channels {
				messages := getMessages(token, channel.ID)
				if messages == nil {
					fmt.Println("Error: nil messages received")
					continue
				}
				for _, message := range messages {
					if message["content"] == nil {
						currentTime := time.Now()
						timeOnly := currentTime.Format("15:04:05")
						fmt.Println("Skipping message with missing content @ " + timeOnly)
						continue
					}
					entryID := uuid.New().String()
					messageData := map[string]interface{}{
						"id":                     entryID,
						"discordAuthorID":        message["author"].(map[string]interface{})["id"],
						"discordServerID":        guild.ID,
						"discordAuthorUsername":  message["author"].(map[string]interface{})["username"],
						"discordMessageID":       message["id"],
						"discordMessageContent":  message["content"],
						"discordTimestamp":       message["timestamp"],
						"discordEditedTimestamp": message["edited_timestamp"],
					}
					messageBuffer = append(messageBuffer, messageData)
					if len(messageBuffer) >= batchSize {
						writeBatchToFile(messageBuffer)
						messageBuffer = nil // Clear buffer
					}
				}
			}
			// Write remaining messages as a final batch
			if len(messageBuffer) > 0 {
				writeBatchToFile(messageBuffer)
			}
		}
	}
	fmt.Println("User info, guilds, and channels for each token saved to accounts.json.")
	fmt.Println("Message data saved to message_data.json.")
}

func writeBatchToFile(messages []map[string]interface{}) {
	fileMutex.Lock()
	defer fileMutex.Unlock()

	f, err := os.OpenFile("message_data.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening message log file:", err)
		return
	}
	defer f.Close()

	for _, message := range messages {
		data, err := json.Marshal(message)
		if err != nil {
			fmt.Println("Error marshalling message data:", err)
			continue
		}
		data = append(data, '\n') // Add newline after each message
		if _, err := f.Write(data); err != nil {
			fmt.Println("Error writing to message log file:", err)
			continue
		}
	}
	fmt.Printf("Batch of %d messages appended to message_data.json\n", len(messages))
}

func getMessages(token, channelID string) []map[string]interface{} {
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://discord.com/api/v9/channels/%s/messages", channelID), nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return nil
	}
	req.Header.Set("Authorization", token)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return nil
	}
	defer resp.Body.Close()

	var rawMessages json.RawMessage
	var messages []map[string]interface{}

	// Read the raw JSON response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return nil
	}

	// Unmarshal into RawMessage
	err = json.Unmarshal(body, &rawMessages)
	if err != nil {
		fmt.Println("Error decoding JSON response:", err)
		return nil
	}

	// Check if the response is an object or an array
	if rawMessages[0] == '{' {
		// If it's an object, wrap it in an array
		rawMessages = []byte("[" + string(rawMessages) + "]")
	}

	// Decode JSON response
	err = json.Unmarshal(rawMessages, &messages)
	if err != nil {
		fmt.Println("Error decoding JSON response:", err)
		return nil
	}

	return messages
}

// API for checking server info example cause I keep forgetting it cause they don't keep it in the public discord docs: https://discord.com/api/v8/invites/owlsec?with_counts=true

// Function to find and log invites
func filterInvites() {
	messages := parseMessages()
	inviteCodes := extractInviteCodes(messages)
	for _, inviteCode := range inviteCodes {
		guildID, guildName, inviteLink, err := getInviteInfo(inviteCode)
		if err != nil {
			fmt.Printf("Error fetching invite info for invite link '%s': %v\n", inviteLink, err)
			continue
		}
		logInviteInfo(guildID, guildName, inviteLink)
	}
}

// Function to parse messages from the message data file
func parseMessages() []string {
	var messages []string

	file, err := os.Open("message_data.json")
	if err != nil {
		fmt.Println("Error opening message data file:", err)
		return messages
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var message map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			fmt.Println("Error decoding message:", err)
			continue
		}
		if message["discordMessageContent"] != nil {
			messages = append(messages, message["discordMessageContent"].(string))
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Error scanning message data file:", err)
	}
	return messages
}

// Function to extract invite codes from messages
func extractInviteCodes(messages []string) []string {
	var inviteCodes []string
	linkRegex := regexp.MustCompile(`(?:https?://)?discord(?:(?:app)?\.com/invite|\.gg)/(\w{5,32})`)

	for _, msg := range messages {
		links := linkRegex.FindAllString(msg, -1)
		for _, link := range links {
			parts := strings.Split(link, "/")
			inviteCodes = append(inviteCodes, parts[len(parts)-1])
		}
	}
	return inviteCodes
}

// Function to retrieve invite information from the Discord API
func getInviteInfo(inviteCode string) (string, string, string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://discord.com/api/v8/invites/%s?with_counts=true", inviteCode), nil)
	if err != nil {
		return "", "", "", err
	}

	// Add headers to mimic a real browser request to bypass rate limits
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3")

	for attempt := 1; attempt <= 5; attempt++ { // Maximum 5 retry attempts
		resp, err := client.Do(req)
		if err != nil {
			return "", "", "", err
		}

		if resp.StatusCode == http.StatusOK {
			var inviteInfo struct {
				GuildID string `json:"guild_id"`
				Guild   struct {
					Name string `json:"name"`
				} `json:"guild,omitempty"`
				InviteLink string `json:"invite_link"`
			}
			err = json.NewDecoder(resp.Body).Decode(&inviteInfo)
			resp.Body.Close()
			if err != nil {
				return "", "", "", err
			}

			guildID := inviteInfo.GuildID
			guildName := ""
			if inviteInfo.Guild.Name != "" {
				guildName = inviteInfo.Guild.Name
			}

			return guildID, guildName, inviteCode, nil
		} else if resp.StatusCode == http.StatusTooManyRequests {
			// If rate limited, wait before retrying
			backoff := time.Duration(attempt*attempt) * time.Second
			fmt.Printf("Rate limited. Retrying in %v...\n", backoff)
			time.Sleep(backoff)
			continue
		} else {
			resp.Body.Close()
			return "", "", "", fmt.Errorf("non-OK status code: %d", resp.StatusCode)
		}
	}

	return "", "", "", fmt.Errorf("maximum retry attempts reached")
}

// Function to log invite information into invites.jsonl
func logInviteInfo(guildID, guildName, inviteCode string) {
	fileMutex.Lock()
	defer fileMutex.Unlock()

	inviteLink := fmt.Sprintf("https://discord.gg/%s", inviteCode)

	inviteData := map[string]interface{}{
		"guild_id":    guildID,
		"guild_name":  guildName,
		"invite_link": inviteLink,
	}

	data, err := json.Marshal(inviteData)
	if err != nil {
		fmt.Println("Error marshalling invite info:", err)
		return
	}

	f, err := os.OpenFile("invites.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening invites log file:", err)
		return
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		fmt.Println("Error writing invite info to file:", err)
		return
	}
}

// Function to manually add invites
func addInviteManually(inviteCode string) error {
	guildID, guildName, inviteLink, err := getInviteInfo(inviteCode)
	if err != nil {
		return err
	}

	// Prepare invite data
	inviteData := map[string]interface{}{
		"guild_id":    guildID,
		"guild_name":  guildName,
		"invite_link": inviteLink,
	}

	// Marshal invite data
	data, err := json.Marshal(inviteData)
	if err != nil {
		return err
	}

	// Open invites.jsonl file
	f, err := os.OpenFile("invites.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write invite data to file
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}

	fmt.Println("Invite added successfully!")
	return nil
}

// Function to remove inactive invites by checking the Discord API
func removeInactiveInvites() error {
	// Open the invites.jsonl file
	file, err := os.Open("invites.jsonl")
	if err != nil {
		return fmt.Errorf("error opening invites file: %v", err)
	}
	defer file.Close()

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	// Define a slice to store the invites
	var invites []map[string]string

	// Iterate over each line in the file
	for scanner.Scan() {
		// Decode the JSON object from the line
		var invite map[string]string
		if err := json.Unmarshal(scanner.Bytes(), &invite); err != nil {
			return fmt.Errorf("error decoding invites data: %v", err)
		}

		// Append the invite to the slice
		invites = append(invites, invite)
	}

	// Check for any scanner errors
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading invites file: %v", err)
	}

	// Iterate over the invites
	for i := 0; i < len(invites); i++ {
		// Extract invite details
		inviteLink := invites[i]["invite_link"]

		// Check invite status using the Discord API
		active, err := isInviteActive(inviteLink)
		if err != nil {
			// Print error message and continue if there's an error
			fmt.Printf("Error fetching invite info for invite link '%s': %v\n", inviteLink, err)
			continue
		}

		// If the invite is not active, remove it from the slice
		if !active {
			fmt.Printf("Removing inactive invite: %s\n", inviteLink)
			invites = append(invites[:i], invites[i+1:]...)
			i-- // Decrement i to handle the removed element
		} else {
			// If the invite is active, print a message
			fmt.Printf("Invite is active: %s\n", inviteLink)
		}
	}

	// Rewrite the invites.jsonl file with the updated data
	file, err = os.Create("invites.jsonl")
	if err != nil {
		return fmt.Errorf("error creating invites file: %v", err)
	}
	defer file.Close()

	// Write each invite as a JSON object on a new line
	for _, invite := range invites {
		if err := json.NewEncoder(file).Encode(invite); err != nil {
			return fmt.Errorf("error encoding invites data: %v", err)
		}
	}

	return nil
}

// Function to check if an invite is active using the Discord API
func isInviteActive(inviteLink string) (bool, error) {
	// Extract the invite code from the invite link
	parts := strings.Split(inviteLink, "/")
	inviteCode := parts[len(parts)-1]

	// Send a GET request to the Discord API to retrieve invite information
	resp, err := http.Get(fmt.Sprintf("https://discord.com/api/v8/invites/%s?with_counts=true", inviteCode))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// Check for non-OK status codes
	if resp.StatusCode == http.StatusNotFound {
		return false, nil // Invite link not found
	} else if resp.StatusCode == http.StatusTooManyRequests {
		return false, fmt.Errorf("rate limited") // Rate limited, handle accordingly
	} else if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("non-OK status code: %d", resp.StatusCode)
	}

	// If the response status code is OK, return true
	return true, nil
}

func joinMissingGuilds() error {
	// Read accounts.json file
	accountsFile, err := os.Open("accounts.json")
	if err != nil {
		return fmt.Errorf("failed to open accounts.json: %v", err)
	}
	defer accountsFile.Close()

	var accountsData AccountData
	err = json.NewDecoder(accountsFile).Decode(&accountsData)
	if err != nil {
		return fmt.Errorf("failed to decode accounts.json: %v", err)
	}

	// Read invites.jsonl file
	invitesFile, err := os.Open("invites.jsonl")
	if err != nil {
		return fmt.Errorf("failed to open invites.jsonl: %v", err)
	}
	defer invitesFile.Close()

	var invitesData []InviteInfo
	scanner := bufio.NewScanner(invitesFile)
	for scanner.Scan() {
		var invite InviteInfo
		err := json.Unmarshal(scanner.Bytes(), &invite)
		if err != nil {
			fmt.Printf("Error parsing JSON line in invites.jsonl: %v\n", err)
			continue
		}
		invitesData = append(invitesData, invite)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning invites.jsonl: %v", err)
	}

	// Track guilds that have been successfully joined
	joinedGuilds := make(map[string]bool)

	// Iterate over each account
	for token, account := range accountsData {
		// Check if the account is in every guild listed in invites.jsonl
		for _, invite := range invitesData {
			if !isInGuild(account, invite.GuildID) && !joinedGuilds[invite.GuildID] {
				// Account is not in this guild and guild has not been joined yet, join using the invite link
				if err := joinServer(token, invite.InviteLink); err != nil {
					fmt.Printf("Error joining server %s: %v\n", invite.GuildName, err)
				} else {
					fmt.Printf("Joined server %s successfully\n", invite.GuildName)
					// Mark guild as joined to prevent duplicate joins
					joinedGuilds[invite.GuildID] = true
				}
			} else {
				fmt.Printf("Already a member of server %s\n", invite.GuildName)
			}
		}
	}

	return nil
}

func isInGuild(account interface{}, guildID string) bool {
	// Check if the account is valid and contains guilds information
	acc, ok := account.(map[string]interface{})
	if !ok {
		return false
	}
	guildsData, ok := acc["guilds"].(map[string]interface{})
	if !ok {
		return false
	}

	// Check if the guild ID is present in the guilds section
	_, exists := guildsData[guildID]
	return exists
}

func joinServer(token, inviteLink string) error {
	// Create a new POST request
	req, err := http.NewRequest("POST", inviteLink, nil)
	if err != nil {
		return err
	}

	// Set the Authorization header with the user token
	req.Header.Set("Authorization", token)

	// Send the POST request using the global HTTP client
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to join server, status code: %d", resp.StatusCode)
	}

	fmt.Println("Successfully joined the server!")

	return nil
}

// Main menu function
func mainMenu() {
	var choice string
	for {
		fmt.Println("\nMain Menu:")
		fmt.Println("1. Account Management")
		fmt.Println("2. Invite Handling")
		fmt.Println("3. Scraping Options")
		fmt.Println("4. Exit")
		fmt.Print("Enter your choice: ")
		_, err := fmt.Scan(&choice)
		if err != nil {
			fmt.Println("Error reading choice:", err)
			continue
		}
		switch choice {
		case "1":
			accountMenu()
		case "2":
			inviteMenu()
		case "3":
			scrapingMenu()
		case "4":
			os.Exit(0)
		default:
			fmt.Println("Invalid choice. Please enter a valid option.")
		}
	}
}

// Account menu function
func accountMenu() {
	var choice string
	for {
		fmt.Println("\nAccount Management:")
		fmt.Println("1. Add Token")
		fmt.Println("2. Back to Main Menu")
		fmt.Print("Enter your choice: ")
		_, err := fmt.Scan(&choice)
		if err != nil {
			fmt.Println("Error reading choice:", err)
			continue
		}
		switch choice {
		case "1":
			var token string
			fmt.Print("Enter the token to add: ")
			_, err := fmt.Scan(&token)
			if err != nil {
				fmt.Println("Error reading token:", err)
				continue
			}
			addToken(strings.TrimSpace(token))
		case "2":
			return
		default:
			fmt.Println("Invalid choice. Please enter a valid option.")
		}
	}
}

func inviteMenu() {
	var choice string
	for {
		fmt.Println("\nInvite Handling:")
		fmt.Println("1. Add Invite Manually")
		fmt.Println("2. Filter Invites")
		fmt.Println("3. Check Invite Statuses")
		fmt.Println("4. Join Missing Guilds")
		fmt.Println("5. Back to Main Menu")
		fmt.Print("Enter your choice: ")
		_, err := fmt.Scan(&choice)
		if err != nil {
			fmt.Println("Error reading choice:", err)
			continue
		}
		switch choice {
		case "1":
			var inviteCode string
			fmt.Print("Enter the invite code to add: ")
			_, err := fmt.Scan(&inviteCode)
			if err != nil {
				fmt.Println("Error reading invite code:", err)
				continue
			}
			addInviteManually(inviteCode)
		case "2":
			filterInvites()
		case "3":
			fmt.Println("\nChecking invite statuses...")
			if err := removeInactiveInvites(); err != nil {
				fmt.Println("Error checking invite statuses:", err)
			} else {
				fmt.Println("Invite statuses checked successfully.")
			}
		case "4":
			fmt.Println("\nJoining missing guilds...")
			if err := joinMissingGuilds(); err != nil {
				fmt.Println("Error joining missing guilds:", err)
			} else {
				fmt.Println("Joining missing guilds completed successfully.")
			}
		case "5":
			return
		default:
			fmt.Println("Invalid choice. Please enter a valid option.")
		}
	}
}

// Scraping menu function
func scrapingMenu() {
	var choice string
	for {
		fmt.Println("\nScraping Options:")
		fmt.Println("1. Scrape Data")
		fmt.Println("2. Back to Main Menu")
		fmt.Print("Enter your choice: ")
		_, err := fmt.Scan(&choice)
		if err != nil {
			fmt.Println("Error reading choice:", err)
			continue
		}
		switch choice {
		case "1":
			scrapeData()
		case "2":
			return
		default:
			fmt.Println("Invalid choice. Please enter a valid option.")
		}
	}
}

// Main function
func main() {
	mainMenu()
}

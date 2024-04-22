package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	_ "time"
)

var (
	httpClient *http.Client
	fileMutex  sync.Mutex
)

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

func scrapData() {
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

func main() {
	var choice string
	for {
		fmt.Println("\nMenu:")
		fmt.Println("1. Add token")
		fmt.Println("2. Scrap data")
		fmt.Println("3. Exit")
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
			scrapData()
		case "3":
			os.Exit(0)
		default:
			fmt.Println("Invalid choice. Please enter a valid option.")
		}
	}
}

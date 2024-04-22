package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid" // Import the uuid package
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	_ "time"
)

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
	// Read existing data from accounts.json
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

	// Write updated data to accounts.json
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
	// Read existing data from accounts.json
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
			for _, channel := range channels {
				messages := getMessages(token, channel.ID)
				for _, message := range messages {
					entryID := uuid.New().String()
					messageData := map[string]interface{}{
						"id":              entryID,
						"authorID":        message["author"].(map[string]interface{})["id"],
						"serverID":        guild.ID,
						"authorUsername":  message["author"].(map[string]interface{})["username"],
						"messageID":       message["id"],
						"messageContent":  message["content"],
						"timestamp":       message["timestamp"],
						"editedTimestamp": message["edited_timestamp"],
					}
					data, err := json.Marshal(messageData)
					if err != nil {
						fmt.Println("Error marshalling message data:", err)
						continue
					}
					err = ioutil.WriteFile("message_data.json", append(data, '\n'), 0644)
					if err != nil {
						fmt.Println("Error writing message data:", err)
						continue
					}
				}
			}
		}
	}
	fmt.Println("User info, guilds, and channels for each token saved to accounts.json.")
	fmt.Println("Message data saved to message_data.json.")
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
	var messages []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&messages)
	if err != nil {
		fmt.Println("Error decoding response:", err)
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

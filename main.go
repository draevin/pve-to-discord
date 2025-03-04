package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Setup http server
	echo := echo.New()

	echo.Use(middleware.Logger())
	echo.Use(middleware.Recover())

	echo.Static("/logs", "logs")

	echo.POST("/webhook", webhook)

	echo.GET("/count", count)

	echo.Logger.Info(echo.Start(":80"))

}

type webhookRequest struct {
	DiscordWebhook   string
	MessageContent   string
	UrlLogAccessible string
	Severity         string
	Title            string
}

type discordWebhook struct {
	Username string  `json:"username"`
	Content  string  `json:"content"`
	Embeds   []Embed `json:"embeds"`
}

type Footer struct {
	Text string `json:"text"`
}

type Author struct {
	Name    string `json:"name"`
	IconUrl string `json:"icon_url"`
}

type Embed struct {
	Author      Author `json:"author"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Footer      Footer `json:"footer"`
}

func webhook(ctx echo.Context) error {

	// Decode JSON from request
	jsonBody := make(map[string]interface{})
	err := json.NewDecoder(ctx.Request().Body).Decode(&jsonBody)
	if err != nil {
		log.Printf("Failed to decode webhook request json: %s", err)
		return ctx.String(http.StatusBadRequest, err.Error())
	}

	webhookrequest := webhookRequest{
		DiscordWebhook:   jsonBody["discordWebhook"].(string),
		MessageContent:   jsonBody["messageContent"].(string),
		UrlLogAccessible: jsonBody["urlLogAccessible"].(string),
		Title:            jsonBody["messageTitle"].(string),
	}

	var embedColor = "15036416"
	var description string

	// Check if Message Content can fit in the embed without needing to summerize it
	if len(webhookrequest.MessageContent) < 4096 && !strings.Contains(webhookrequest.Title, "vzdump") {
		description = fmt.Sprintf("```%s```", webhookrequest.MessageContent)
	} else {
		summary := summarizeMessageContent(webhookrequest.MessageContent)

		// Save file to disk
		fileName, err := saveLogToDisk(&webhookrequest)
		if err != nil {
			log.Printf("Failed to write log file: %s", err)
			ctx.String(http.StatusBadRequest, err.Error())
		}

		// Add Summary to embed discription
		if len(summary) > 1 {
			description = fmt.Sprintf("```%s``` You can find the detailed log [here](%s%s)", summary, webhookrequest.UrlLogAccessible, fileName)
		} else {
			description = fmt.Sprintf("You can find the detailed log [here](%s%s) ", webhookrequest.UrlLogAccessible, fileName)
		}
	}

	// Craft Payload
	var iconUrl = "https://pbs.proxmox.com/docs/_static/favicon.ico"
	var authorName = "Proxmox VE"
	author := Author{
		Name:    authorName,
		IconUrl: iconUrl,
	}

	var footerText = "Ordis via pvetodiscord"
	footer := Footer{
		Text: footerText,
	}

	embed := Embed{
		Author:      author,
		Title:       webhookrequest.Title,
		Description: description,
		Color:       embedColor,
		Footer:      footer,
	}

	discordPayload := discordWebhook{
		Username: "Ordis",
		Content:  "",
		Embeds:   []Embed{embed},
	}

	payload := new(bytes.Buffer)

	err = json.NewEncoder(payload).Encode(discordPayload)
	if err != nil {
		log.Printf("Failed to encode json: %s", err)
		ctx.String(http.StatusBadRequest, err.Error())
	}

	// Send payload to discord
	resp, err := http.Post(webhookrequest.DiscordWebhook, "application/json", payload)
	if err != nil {
		log.Printf("Failed to send discord webhook: %s", err)
	}

	// Check response from discord for errors
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ctx.String(http.StatusBadRequest, string(responseBody))
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return ctx.String(http.StatusBadRequest, string(responseBody))
	}

	return ctx.String(http.StatusOK, string(responseBody))
}

func saveLogToDisk(webhookRequest *webhookRequest) (string, error) {
	time := time.Now()
	filename := fmt.Sprintf("%s.log", time.Format("2006-01-02.15-04-05"))

	err := os.WriteFile(fmt.Sprintf("logs/%s", filename), []byte(webhookRequest.MessageContent), 0644)

	log.Printf("Log file %s written to disk", filename)

	return filename, err
}

func summarizeMessageContent(data string) string {
	lines := strings.Split(data, "\n")

	var cleanedLines []string
	var totalRunningTime string
	var totalSize string

	// Process each line to split and clean the necessary data
	for _, line := range lines {
		// Trim any leading/trailing spaces
		line = strings.TrimSpace(line)

		// Stop processing if "Logs" section is encountered
		if strings.HasPrefix(line, "Logs") {
			break
		}

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		// Skip "Details" header
		if line == "Details" {
			continue
		}

		// Skip Headers
		if strings.Contains(line, "VMID") {
			continue
		}

		// Capture "Total running time" and "Total size"
		if strings.HasPrefix(line, "Total running time:") {
			totalRunningTime = line
			continue
		}
		if strings.HasPrefix(line, "Total size:") {
			totalSize = line
			continue
		}

		// Split the line by spaces, expecting at least 4 columns
		columns := strings.Fields(line)

		// Check if there are enough columns (we expect at least 4)
		if len(columns) < 4 {
			continue // Skip rows that don't have enough data
		}

		// Remove the "Size" and "Filename" columns by trimming the slice
		columns = columns[:4]

		// Build the cleaned line and align it properly
		cleanedLine := fmt.Sprintf("%-5s %-25s %-8s %-10s",
			columns[0], columns[1], columns[2], columns[3])

		// Add the cleaned line to the result
		cleanedLines = append(cleanedLines, cleanedLine)
	}

	// Add a blank line before the totals
	cleanedLines = append(cleanedLines, "")

	// Append the "Total running time" and "Total size" directly
	if totalRunningTime != "" {
		cleanedLines = append(cleanedLines, totalRunningTime)
	}
	if totalSize != "" {
		cleanedLines = append(cleanedLines, totalSize)
	}

	// Format the header correctly (only once)
	header := fmt.Sprintf("%-5s %-25s %-8s %-10s", "VMID", "Name", "Status", "Time")

	// Combine everything into one string
	finalOutput := strings.Join(append([]string{header}, cleanedLines...), "\n")

	// Return the final formatted string
	return finalOutput
}

func count(ctx echo.Context) error {
	logs, err := os.ReadDir("/logs")

	log.Printf("Retrieved %d files for status check", len(logs))

	if err != nil {
		return ctx.String(http.StatusInternalServerError, err.Error())
	}

	return ctx.String(http.StatusOK, fmt.Sprintf("logs: %d", len(logs)))
}

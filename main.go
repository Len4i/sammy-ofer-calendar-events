package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const (
	title  = "Sammy Ofer Football match"
	url    = "https://www.haifa-stadium.co.il/%d7%9c%d7%95%d7%97_%d7%94%d7%9e%d7%a9%d7%97%d7%a7%d7%99%d7%9d_%d7%91%d7%90%d7%a6%d7%98%d7%93%d7%99%d7%95%d7%9f/"
	domain = "www.haifa-stadium.co.il"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "/tmp/token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func main() {

	c := colly.NewCollector(
		colly.AllowedDomains(domain),
	)

	dates := make(map[string]bool)
	c.OnHTML(".elementor-column", func(e *colly.HTMLElement) {
		dateText := e.ChildText(".elementor-widget-container")
		dateRegex := regexp.MustCompile(`\b\d{1,2}/\d{1,2}/\d{2}\b`)
		// timeRegex := regexp.MustCompile(`\b\d{2}:\d{2}\b`)

		dateMatch := dateRegex.FindString(dateText)
		// timeMatch := timeRegex.FindString(dateText)

		// if dateMatch != "" && timeMatch != "" {
		// 	dates = append(dates, fmt.Sprintf("%s %s", dateMatch, timeMatch))
		// }
		if dateMatch != "" {
			dates[dateMatch] = true
		}
	})

	c.Visit(url)

	fmt.Println(dates)
	ctx := context.Background()
	b, err := os.ReadFile("creds.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	for d, _ := range dates {
		parsedDate, err := time.Parse("2/1/06", strings.Split(d, " ")[0])
		if err != nil {
			log.Fatalf("Error parsing date: %v", err)
		}

		date := time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, time.Local)

		fmt.Println(date.Format("2006-01-02"))
		if func() bool {
			events, err := srv.Events.List("primary").ShowDeleted(false).
				SingleEvents(true).Q(title).
				TimeMin(date.Format(time.RFC3339)).
				MaxResults(1).OrderBy("startTime").Do()
			if err != nil {
				log.Fatalf("Unable to retrieve next ten of the user's events: %v", err)
			}
			for _, item := range events.Items {
				if item.Summary == title {
					fmt.Println("Event already present")
					return true
				}
			}
			return false
		}() {
			continue
		}
		event := &calendar.Event{
			Summary: title,
			Start: &calendar.EventDateTime{
				Date:     date.Format("2006-01-02"),
				TimeZone: "Asia/Jerusalem",
			},
			End: &calendar.EventDateTime{
				Date:     date.Add(24 * time.Hour).Format("2006-01-02"),
				TimeZone: "Asia/Jerusalem",
			},
		}

		calendarId := "primary"
		event, err = srv.Events.Insert(calendarId, event).Do()
		if err != nil {
			log.Fatalf("Unable to create event. %v\n", err)
		}
		fmt.Printf("Event created: %s\n", event.HtmlLink)
	}
}

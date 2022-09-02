package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
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
	url    = "https://www.haifa-stadium.com/schedule_of_matches_in_the_stadium/"
	domain = "www.haifa-stadium.com"
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
	r, _ := regexp.Compile(`\d+\/\d+\s[0-9:]+`)

	var dates []string
	//c.OnHTML("body > div.elementor.elementor-284 > div > div > section.elementor-section.elementor-top-section.elementor-element.elementor-element-b31ab00.elementor-section-boxed.elementor-section-height-default.elementor-section-height-default > div > div > div > div > div > section > div > div > div.elementor-column.elementor-col-25.elementor-inner-column.elementor-element.elementor-element-8129b72 > div > div > div > div > div > p", func(e *colly.HTMLElement) {
	c.OnHTML(".elementor-section-wrap", func(e *colly.HTMLElement) {
		e.ForEach(".elementor-section.elementor-top-section.elementor-element", func(_ int, el *colly.HTMLElement) {
			found := el.ChildText(".elementor-text-editor.elementor-clearfix")
			match := r.FindString(found)
			if strings.TrimSpace(match) != "" {
				dates = append(dates, strings.Split(strings.TrimSpace(match), " ")[0])
			}
		})
	})

	c.Visit(url)

	ctx := context.Background()
	b, err := os.ReadFile("/tmp/creds.json")
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

	var year, day string
	yearNow := time.Now().Year()
	monthNow := int(time.Now().Month())
	for _, d := range dates {
		day = strings.Split(d, "/")[0]
		month, err := strconv.Atoi(strings.Split(d, "/")[1])
		if err != nil {
			log.Fatalf("can't convert month from scrapper to int: %v", err)
		}
		if month < monthNow {
			i := yearNow + 1
			year = strconv.Itoa(i)

		} else {
			year = strconv.Itoa(yearNow)
		}

		fmt.Println(year + "-" + strconv.Itoa(month) + "-" + day)
		if func() bool {
			events, err := srv.Events.List("primary").ShowDeleted(false).
				SingleEvents(true).Q(title).
				TimeMin(year + "-" + strconv.Itoa(month) + "-" + day + "T00:00:00+03:00").
				MaxResults(10).OrderBy("startTime").Do()
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
				Date:     year + "-" + strconv.Itoa(month) + "-" + day,
				TimeZone: "Asia/Jerusalem",
			},
			End: &calendar.EventDateTime{
				Date:     year + "-" + strconv.Itoa(month) + "-" + day,
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

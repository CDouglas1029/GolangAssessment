package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"golang.org/x/time/rate"
)

type Post struct {
	Title   string
	Author  string
	Upvotes int
}

var (
	mostUpvotedPosts []Post
	userPostCount    map[string]int
	mu               sync.Mutex
	rateLimiter      = rate.NewLimiter(1, 5) // 1 request per second with a burst of 5
	client           = resty.New()
)

func init() {
	client.SetBaseURL("https://oauth.reddit.com")
	client.SetHeader("User-Agent", os.Getenv("REDDIT_USER_AGENT"))
	client.SetAuthToken(getRedditAuthToken())
	userPostCount = make(map[string]int)
}

func main() {
	subreddit := "golang" // Choose a subreddit of interest
	go fetchPosts(subreddit)

	// Log stats every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		logStatistics()
	}
}

func getRedditAuthToken() string {
	resp, err := resty.New().
		SetBaseURL("https://www.reddit.com").
		SetBasicAuth(os.Getenv("REDDIT_CLIENT_ID"), os.Getenv("REDDIT_CLIENT_SECRET")).
		SetHeader("User-Agent", os.Getenv("REDDIT_USER_AGENT")).
		R().
		SetFormData(map[string]string{"grant_type": "client_credentials"}).
		Post("/api/v1/access_token")

	if err != nil {
		log.Fatalf("Failed to authenticate: %v", err)
	}

	// Check if the response is an error
	if resp.IsError() {
		log.Fatalf("Authentication request failed with status %v: %v", resp.Status(), resp)
	}

	// Parse response body safely
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		log.Fatalf("Failed to parse authentication response: %v", err)
	}

	// Ensure access_token exists in response
	accessToken, ok := result["access_token"].(string)
	if !ok {
		log.Fatalf("No access token found in response: %v", result)
	}

	return accessToken
}

func fetchPosts(subreddit string) {
	for {
		if err := rateLimiter.Wait(context.Background()); err != nil {
			log.Printf("Rate limit wait error: %v", err)
			continue
		}

		resp, err := client.R().Get(fmt.Sprintf("/r/%s/new", subreddit))
		if err != nil {
			log.Printf("Failed to fetch posts: %v", err)
			continue
		}

		if resp.IsError() {
			log.Printf("API returned error: %v", resp.Status())
			continue
		}

		// Process response
		var posts []Post
		// Extract post data here (parsing JSON omitted for simplicity)

		updateStatistics(posts)
	}
}

func updateStatistics(posts []Post) {
	mu.Lock()
	defer mu.Unlock()

	for _, post := range posts {
		if len(mostUpvotedPosts) < 10 || post.Upvotes > mostUpvotedPosts[9].Upvotes {
			mostUpvotedPosts = append(mostUpvotedPosts, post)
			// Sort mostUpvotedPosts by Upvotes (sorting code omitted)
		}

		userPostCount[post.Author]++
	}
}

func logStatistics() {
	mu.Lock()
	defer mu.Unlock()

	fmt.Println("Most Upvoted Posts:")
	for _, post := range mostUpvotedPosts {
		fmt.Printf("Title: %s, Author: %s, Upvotes: %d\n", post.Title, post.Author, post.Upvotes)
	}

	fmt.Println("\nTop Users:")
	for user, count := range userPostCount {
		fmt.Printf("User: %s, Posts: %d\n", user, count)
	}
}

package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/IchWambo/Blog_Aggregator/internal/config"
	"github.com/IchWambo/Blog_Aggregator/internal/database"
	"github.com/google/uuid"

	_ "github.com/lib/pq"
)

func main() {

	cfg := config.Read()
	//fmt.Println("URL:", cfg.DB_URL)
	//fmt.Println("User Name:", cfg.Current_User_Name)
	var comms Commands
	comms.commandsMap = make(map[string]func(*State, Command) error)
	comms.register("login", handlerLogin)
	comms.register("register", handlerRegister)
	comms.register("reset", handlerReset)
	comms.register("users", handlerGetUsers)
	comms.register("agg", fetchFeed)
	comms.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	comms.register("feeds", handlerFeeds)
	comms.register("follow", middlewareLoggedIn(follow))
	comms.register("following", following)
	comms.register("unfollow", middlewareLoggedIn(unfollow))
	comms.register("browse", browse)

	args := os.Args
	if len(args) < 2 {
		log.Fatal("expected at least 2 arguments")
	}

	var command Command
	command.name = args[1]
	command.args = args[2:]

	db, err := sql.Open("postgres", cfg.DB_URL)
	if err != nil {
		log.Fatal("error opening connection with postgres: ", err)
	}

	dbQueries := database.New(db)

	states := &State{
		db:  dbQueries,
		cfg: cfg,
	}

	err = comms.run(states, command)
	if err != nil {
		log.Fatal(err)
	}

}

type State struct {
	db  *database.Queries
	cfg *config.Config
}

type Command struct {
	name string
	args []string
}

func handlerLogin(s *State, cmd Command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("expected username, found none")
	}

	if _, err := s.db.GetUser(context.Background(), cmd.args[0]); err != nil {
		fmt.Printf("username doesn't exist: %v", cmd.args[0])
		os.Exit(1)
	}

	s.cfg.Current_User_Name = cmd.args[0]
	err := s.cfg.SetUser(cmd.args[0])
	if err != nil {
		return err
	}

	fmt.Printf("set username to: %v\n", cmd.args[0])

	return nil
}

type Commands struct {
	commandsMap map[string]func(*State, Command) error
}

func (c *Commands) run(s *State, cmd Command) error {
	val, ok := c.commandsMap[cmd.name]
	if !ok {
		return fmt.Errorf("Command not found: %v", cmd.name)
	}
	return val(s, cmd)
}

func (c *Commands) register(name string, f func(s *State, cmd Command) error) {
	c.commandsMap[name] = f

}

func handlerRegister(s *State, cmd Command) error {
	if len(cmd.args[0]) < 1 {
		return fmt.Errorf("expected name, found none")
	}

	user, err := s.db.CreateUser(context.Background(), database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Name:      cmd.args[0],
	},
	)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			fmt.Printf("user name already exists: %v", cmd.args[0])
			os.Exit(1)
		} else {
			return fmt.Errorf("error creating user: %v", err)
		}
	}

	s.cfg.Current_User_Name = cmd.args[0]
	err = s.cfg.SetUser(cmd.args[0])
	if err != nil {
		return err
	}

	fmt.Println("creater user successfully")
	log.Printf("%+v registered\n", user.Name)

	return nil
}

func handlerReset(s *State, cmd Command) error {

	err := s.db.Reset(context.Background())
	if err != nil {
		fmt.Printf("error reseting: %v", err)
		os.Exit(1)
	}

	fmt.Println("reset successfully")
	return nil
}

func handlerGetUsers(s *State, cmd Command) error {

	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		fmt.Printf("error getting users: %v\n", err)
		os.Exit(1)
	}

	for _, user := range users {
		if user == s.cfg.Current_User_Name {
			fmt.Printf("%v (current)\n", user)
		} else {
			fmt.Println(user)
		}
	}
	return nil
}

func fetchFeed(s *State, cmd Command) error {

	if len(cmd.args) != 1 {
		return fmt.Errorf("expecting arguments like: 1s, 1m")
	}
	tick_time := cmd.args[0]

	parse_time, err := time.ParseDuration(tick_time)
	if err != nil {
		return fmt.Errorf("error parsing time duration: %v\n", err)
	}

	ticker := time.NewTicker(parse_time)
	fmt.Printf("Collecting feed every %v.\n", parse_time)

	counter := 0
	for ; ; <-ticker.C {
		counter++
		fmt.Printf("tick %d\n", counter)

		if err := scrapeFeeds(s, cmd); err != nil {
			if !strings.Contains(err.Error(), "posts_url_key") {
				fmt.Printf("error scraping feeds: %v", err)
			}
		}
	}
}

func handlerFetchFeed(url string) (*RSSFeed, error) {

	feedURL := url
	req, err := http.NewRequest("GET", feedURL, nil)
	if err != nil {
		fmt.Printf("error declaring request: %v\n", err)
	}
	req.Header.Set("User-Agent", "aggregator")

	client := &http.Client{}

	res, err := client.Do(req)
	if err != nil {
		fmt.Printf("error fulfilling request: %v\n", err)
	}

	defer res.Body.Close()

	reader, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("error reading request: %v\n", err)
	}

	var rssFeed RSSFeed

	unmarshal := xml.Unmarshal(reader, &rssFeed)
	if unmarshal != nil {
		fmt.Printf("error unmarshaling data: %v\n", unmarshal)
	}

	html.UnescapeString(rssFeed.Channel.Title)
	html.UnescapeString(rssFeed.Channel.Description)

	for _, item := range rssFeed.Channel.Item {
		html.UnescapeString(item.Title)
		html.UnescapeString(item.Description)
	}

	fmt.Printf("fetched %d posts from %s\n", len(rssFeed.Channel.Item), feedURL)

	return &rssFeed, nil
}

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func handlerAddFeed(s *State, cmd Command, user database.User) error {

	feed, err := s.db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Name:      cmd.args[0],
		Url:       cmd.args[1],
		UserID:    user.ID,
	},
	)

	if err != nil {
		fmt.Printf("error creating feed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully created feed %v.\n", feed.Name)

	_, feedErr := s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	},
	)

	if feedErr != nil {
		fmt.Printf("error creating feed follow: %v\n", feedErr)
	}

	return nil
}

func handlerFeeds(s *State, cmd Command) error {

	fmt.Println(s.db.GetFeeds(context.Background()))

	return nil
}

func follow(s *State, cmd Command, user database.User) error {

	if len(cmd.args) != 1 {
		fmt.Printf("one argument permitted / required\n")
		os.Exit(1)
	}

	feed, err := s.db.GetFeedByUrl(context.Background(), cmd.args[0])
	if err != nil {
		fmt.Printf("error getting feed: %v: %v\n", cmd.args[0], err)
		os.Exit(1)
	}

	_, err = s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	},
	)

	if err != nil {
		fmt.Printf("error creating follow: %v", err)
		os.Exit(1)
	}

	fmt.Println(feed.Name, s.cfg.Current_User_Name)

	return nil
}

func following(s *State, cmd Command) error {

	if len(cmd.args) > 1 {
		fmt.Println("only one argument allowed")
		os.Exit(1)
	}

	feeds, err := s.db.GetFeedFollowsForUsers(context.Background(), s.cfg.Current_User_Name)
	if err != nil {
		fmt.Printf("error getting feed follows for user: %v , %v\n", s.cfg.Current_User_Name, err)
		os.Exit(1)
	}

	for feed := range feeds {
		fmt.Println(feeds[feed].Name_2)
	}

	return nil
}

func middlewareLoggedIn(handler func(s *State, cmd Command, user database.User) error) func(*State, Command) error {

	return func(s *State, cmd Command) error {

		user, err := s.db.GetUser(context.Background(), s.cfg.Current_User_Name)
		if err != nil {
			return fmt.Errorf("error getting user for middleware login: %v\n", err)
		}

		return handler(s, cmd, user)
	}
}

func unfollow(s *State, cmd Command, user database.User) error {

	if len(cmd.args) > 1 {
		fmt.Println("only one argument allowed")
		os.Exit(1)
	}

	feed, err := s.db.GetFeedByUrl(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("error gettinf feed by url for unfollow: %v\n", err)
	}

	err = s.db.DeleteFeed(context.Background(), database.DeleteFeedParams{
		UserID: user.ID,
		FeedID: feed.ID,
	},
	)

	if err != nil {
		return fmt.Errorf("error deleting feed_follow: \nuser id: %v\nfeed id: %v\n%v\n", user.ID, feed.ID, err)
	}

	fmt.Println("Successfully unfollowed feed.")

	return nil
}

func scrapeFeeds(s *State, cmd Command) error {

	next_feed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return fmt.Errorf("error getting next feed to fetch: %v\n", err)
	}

	fmt.Printf("scraping feed: %s (%s)\n", next_feed.Name, next_feed.Url)

	err = s.db.MarkFeedFetched(context.Background(), next_feed.ID)
	if err != nil {
		return fmt.Errorf("error marking feed as fetched: %v\n", err)
	}

	rssFeed, err := handlerFetchFeed(next_feed.Url)
	if err != nil {
		return fmt.Errorf("error fetching feed: %v\n", err)
	}

	fmt.Println("number of items in rssFeed:", len(rssFeed.Channel.Item))

	skipped := 0
	created := 0

	for _, post := range rssFeed.Channel.Item {

		parsed_time, err := time.Parse(time.RFC1123, post.PubDate)
		var publishedAt sql.NullTime
		if err == nil {
			publishedAt = sql.NullTime{Time: parsed_time, Valid: true}
		} else {
			publishedAt = sql.NullTime{Valid: false}
		}

		new_post, err := s.db.CreatePost(context.Background(), database.CreatePostParams{
			ID:          uuid.New(),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
			Title:       post.Title,
			Url:         post.Link,
			Description: post.Description,
			PublishedAt: publishedAt,
			FeedID:      next_feed.ID,
		})

		if err != nil {
			if strings.Contains(err.Error(), "posts_url_key") {
				skipped++
				continue
			}
			fmt.Printf("error creating post %q: %v\n", post.Title, err)
		}
		fmt.Println("successfully created post: ", new_post.Title)
		created++
	}

	fmt.Printf("created %d new posts\nskipped creating %d posts since they already exist\n", created, skipped)

	return nil
}

func browse(s *State, cmd Command) error {

	limit := "2"

	if len(cmd.args) > 1 {
		return fmt.Errorf("usage: browse [limit]")
	}

	if len(cmd.args) == 1 {
		limit = cmd.args[0]
	}

	converted_limit, err := strconv.Atoi(limit)
	if err != nil {
		return fmt.Errorf("error converting limit: %v", err)
	}

	posts, err := s.db.GetPostsForUser(context.Background(), database.GetPostsForUserParams{
		Name:  s.cfg.Current_User_Name,
		Limit: int32(converted_limit),
	})

	fmt.Printf("successfully got %v posts.\n", converted_limit)

	for _, post := range posts {
		fmt.Printf("Title: %v\n", post.Title)
		fmt.Printf("Url: %v\n", post.Url)
	}

	return nil
}

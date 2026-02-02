# Blog Aggregator

This is a simple blog/RSSFeed aggregator written in Go.

## Requirements

-Go
-PostgreSQL

## Installation

Install the `aggregator` CLI using `go install`:

go install github.com/IchWambo/Blog_Aggregator@latest

## Configuration

You need a .gatorconfig.json file so gator knows how to connect to your database and which user is “current”.

Copy the example config file in this repo (e.g. .gatorconfig.example.json) to .gatorconfig.json.
Update it with:
    Your Postgres connection URL
    Your current username

By default, the config file is expected at:

/home/<username>/.gatorconfig.json

If you’re on a different OS, or want to keep it somewhere else, you can change the path in the internal/config/ package.

## Usage

The idea is that you first create one or multiple users and then
add the feeds by their name and link to the user so we can aggregate them with the feed scraper.
This is crucial since the aggregator works on a user and always needs to be set to one.

Here are some examples:

register bob -> registers a new user with the name bob and logs him in as the current user and changes the user in the .gatorconfig file.
addfeed "RSSFeed Name" <link> -> adds the RSSFeed and automatically adds a feed_follow for it to the user to scrape the feeds.
agg <time> -> goes through all the feed_follows a user has and aggregates the RSSFeeds every <time> which should be in the format: 1s, 1m, 1h etc...
browse <number> -> gets the <number> latest feeds for the user, the <number> can be omitted and will default to 2.


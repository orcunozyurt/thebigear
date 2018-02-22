package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rekognition"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/joho/godotenv"
	"github.com/thebigear/database"
	"github.com/thebigear/models"
	"github.com/thebigear/utils"
	"github.com/tuvistavie/structomap"
	"mvdan.cc/xurls"
)

func init() {
	database.Connect()
	database.EnsureIndexes()
	// Use snake case in all serializers
	structomap.SetDefaultCase(structomap.SnakeCase)
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file", err)
	}
}

func initRekognitionConnection() (*rekognition.Rekognition, error) {

	sess, err := session.NewSession()
	if err != nil {
		fmt.Println("Error creating session ", err)
		return nil, err
	}

	// Create and return a Rekognition client from just a session.
	return rekognition.New(sess), nil

}

func initTwitterConnection() *twitter.Client {
	fmt.Println("Initializing Twitter Connection...")

	consumerKey := utils.GetEnvOrDefault("TWITTER_CONSUMER_KEY", "")
	consumerSecret := utils.GetEnvOrDefault("TWITTER_CONSUMER_SECRET", "")

	accessToken := utils.GetEnvOrDefault("TWITTER_ACCESS_TOKEN", "")
	accessSecret := utils.GetEnvOrDefault("TWITTER_ACCESS_SECRET", "")

	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)
	httpClient := config.Client(oauth1.NoContext, token)

	return twitter.NewClient(httpClient)

}

func CleanTweet(tweet twitter.Tweet) string {
	text := tweet.FullText
	url := xurls.Relaxed().FindAllString(text, -1)

	if len(url) > 0 {
		for _, element := range url {

			text = strings.Replace(text, element, "", -1)
		}
	}
	if tweet.ExtendedTweet != nil && tweet.ExtendedTweet.Entities != nil {

		hashtags := tweet.ExtendedTweet.Entities.Hashtags
		mentions := tweet.ExtendedTweet.Entities.UserMentions
		medias := tweet.ExtendedTweet.Entities.Media
		urls := tweet.ExtendedTweet.Entities.Urls

		if len(hashtags) > 0 {

			for _, element := range hashtags {

				text = strings.Replace(text, element.Text, "", -1)

			}

		}
		if len(mentions) > 0 {

			for _, element := range mentions {

				text = strings.Replace(text, element.Name, "", -1)

			}

		}
		if len(medias) > 0 {

			for _, element := range medias {

				text = strings.Replace(text, element.MediaURL, "", -1)

			}

		}
		if len(urls) > 0 {

			for _, element := range urls {

				text = strings.Replace(text, element.URL, "", -1)

			}

		}
	}

	text = strings.Replace(text, "_", " ", -1)
	text = strings.Replace(text, "#", "", -1)
	text = strings.Replace(text, "@", "", -1)
	text = strings.Replace(text, "\n", "", -1)
	text = strings.Replace(text, "\r", "", -1)

	// Make a Regex to say we only want
	reg, err := regexp.Compile("[^a-zA-Z0-9 ]+ ")
	if err != nil {
		log.Fatal(err)
	}
	text = reg.ReplaceAllString(text, " ")

	whitespacedeletereg, err := regexp.Compile("[ ]{2,}")
	if err != nil {
		log.Fatal(err)
	}
	text = whitespacedeletereg.ReplaceAllString(text, " ")

	return text

}

func main() {
	key := flag.String("key", "foo", "search key")
	count := flag.Int("count", 100, "Tweet results per page")
	popular := flag.Bool("popular", false, "Want Popular Results")

	flag.Parse()

	fmt.Println("key:", *key)
	fmt.Println("count:", *count)
	fmt.Println("popular:", *popular)

	twClient := initTwitterConnection()
	rekogClient, _ := initRekognitionConnection()

	tweets := GetTweetsFromSearchApi(twClient, key, count, popular)

	for _, tweet := range tweets {

		query := database.Query{}
		query["post_id"] = tweet.ID

		duplicate, _ := models.GetExpression(query)
		totalInteraction := tweet.FavoriteCount + tweet.RetweetCount
		cleanText := CleanTweet(tweet)

		if duplicate == nil && cleanText != "" && totalInteraction > 1 {

			fmt.Print("\nCLEAN TEXT: ", cleanText)
			isVerified := tweet.User.Verified
			hasAtatchments := HasAttachment(tweet)
			followerCount := tweet.User.FollowersCount
			followingCount := tweet.User.FriendsCount
			postsCount := tweet.User.StatusesCount

			expression := &models.Expression{}
			expression.PostID = tweet.ID
			expression.Owner = tweet.User.IDStr
			expression.FullText = tweet.FullText
			expression.CleanText = cleanText
			expression.IsVerified = &isVerified
			expression.HasAttachment = &hasAtatchments
			expression.Followers = &followerCount
			expression.Following = &followingCount
			expression.PostCount = &postsCount

			userTweets := GetUserTweetsFromTimeline(twClient, tweet.User.ID)
			totalLikes := 0
			totalRetweets := 0
			for _, userTweet := range userTweets {
				totalLikes += userTweet.FavoriteCount
				totalRetweets += userTweet.RetweetCount
			}

			lasttentotal := totalLikes + totalRetweets

			expression.LastTenInteraction = &lasttentotal
			expression.TotalInteraction = &totalInteraction

			photoAttached, mediaIndex := IsAnyAttachmentPhoto(tweet)
			if photoAttached == true {

				expression.MediaURL = tweet.Entities.Media[mediaIndex].MediaURL

				imagebytes := DownloadImage(tweet.Entities.Media[mediaIndex].MediaURL)
				labels, err := detectLabels(rekogClient, imagebytes)

				if err == nil {
					expression.AttachmentLabels = &labels
				}

			}

			expression.Create()
		}

	}

}

func GetUserTweetsFromTimeline(client *twitter.Client, userID int64) []twitter.Tweet {

	er := true
	ir := false

	params := &twitter.UserTimelineParams{
		Count:           10,
		ExcludeReplies:  &er,
		IncludeRetweets: &ir,
		UserID:          userID,
	}

	timeline, _, _ := client.Timelines.UserTimeline(params)

	return timeline
}

func GetTweetsFromSearchApi(client *twitter.Client, key *string, count *int, popular *bool) []twitter.Tweet {

	// at least 2 days old tweets
	current_time := time.Now().AddDate(0, 0, -2)
	formatted_time := current_time.Format("2006-01-02")

	ie := true
	rpp := count
	var res_type string
	query := fmt.Sprintf("%s AND -filter:retweets AND -filter:replies", *key)

	fmt.Println(query)

	if *popular {
		res_type = "popular"
	} else {
		res_type = "mixed"
	}
	params := &twitter.SearchTweetParams{
		Query:           query,
		Lang:            "en",
		IncludeEntities: &ie,
		TweetMode:       "extended",
		ResultType:      res_type,
		Count:           *rpp,
		Until:           formatted_time,
	}

	//Search Tweets
	search, _, _ := client.Search.Tweets(params)

	return search.Statuses

}

func HasAttachment(tweet twitter.Tweet) bool {

	return len(tweet.Entities.Media) > 0

}

func IsAnyAttachmentPhoto(tweet twitter.Tweet) (bool, int) {

	if HasAttachment(tweet) == false {
		return false, 99
	}

	for index, media := range tweet.Entities.Media {
		if media.Type == "photo" {
			return true, index
		}
	}

	return false, 99

}

func DownloadImage(url string) []byte {
	res, err := http.Get(url)

	if err != nil {
		log.Fatalf("http.Get -> %v", err)
	}

	// We read all the bytes of the image
	// Types: data []byte
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatalf("ioutil.ReadAll -> %v", err)
	}

	// You have to manually close the body, check docs
	// This is required if you want to use things like
	// Keep-Alive and other HTTP sorcery.
	res.Body.Close()

	return data
}

func detectLabels(svc *rekognition.Rekognition, data []byte) (string, error) {

	mc := float64(60)
	params := &rekognition.DetectLabelsInput{
		Image: &rekognition.Image{ // Required
			Bytes: data,
		},
		MinConfidence: &mc,
	}

	result, err := svc.DetectLabels(params)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case rekognition.ErrCodeInvalidS3ObjectException:
				fmt.Println(rekognition.ErrCodeInvalidS3ObjectException, aerr.Error())
			case rekognition.ErrCodeInvalidParameterException:
				fmt.Println(rekognition.ErrCodeInvalidParameterException, aerr.Error())
			case rekognition.ErrCodeImageTooLargeException:
				fmt.Println(rekognition.ErrCodeImageTooLargeException, aerr.Error())
			case rekognition.ErrCodeAccessDeniedException:
				fmt.Println(rekognition.ErrCodeAccessDeniedException, aerr.Error())
			case rekognition.ErrCodeInternalServerError:
				fmt.Println(rekognition.ErrCodeInternalServerError, aerr.Error())
			case rekognition.ErrCodeThrottlingException:
				fmt.Println(rekognition.ErrCodeThrottlingException, aerr.Error())
			case rekognition.ErrCodeProvisionedThroughputExceededException:
				fmt.Println(rekognition.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case rekognition.ErrCodeInvalidImageFormatException:
				fmt.Println(rekognition.ErrCodeInvalidImageFormatException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return "", err
	}
	var s []string

	if len(result.Labels) > 0 {

		for _, element := range result.Labels {

			s = append(s, *element.Name)

		}

		return strings.Join(s, " "), nil

	}

	return "", errors.New("No Match")

}

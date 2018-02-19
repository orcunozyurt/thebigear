package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"

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

func CleanText(text string) string {

	url := xurls.Relaxed().FindAllString(text, -1)

	if len(url) > 0 {
		for _, element := range url {

			text = strings.Replace(text, element, "", -1)
		}
	}

	// Make a Regex to say we only want
	reg, err := regexp.Compile("[^a-zA-Z0-9 -]+ ")
	if err != nil {
		log.Fatal(err)
	}
	text = reg.ReplaceAllString(text, " ")

	return text

}

func main() {
	twClient := initTwitterConnection()
	rekogClient, _ := initRekognitionConnection()

	tweets := GetTweetsFromSearchApi(twClient)

	for _, tweet := range tweets {
		fmt.Print("FULL TEXT: ", tweet.FullText)
		fmt.Print("CLEAN TEXT: ", CleanText(tweet.FullText))

		expression := &models.Expression{}
		expression.Owner = tweet.User.IDStr
		expression.FullText = tweet.FullText
		expression.CleanText = CleanText(tweet.FullText)
		expression.IsVerified = tweet.User.Verified
		expression.HasAttachment = HasAttachment(tweet)
		expression.Followers = tweet.User.FollowersCount
		expression.Following = tweet.User.FriendsCount
		expression.PostCount = tweet.User.StatusesCount

		userTweets := GetUserTweetsFromTimeline(twClient, tweet.User.ID)
		totalLikes := 0
		totalRetweets := 0
		for _, userTweet := range userTweets {
			totalLikes += userTweet.FavoriteCount
			totalRetweets += userTweet.RetweetCount
		}

		expression.LastTenInteraction = totalLikes + totalRetweets
		expression.TotalInteraction = tweet.FavoriteCount + tweet.RetweetCount

		photoAttached, mediaIndex := IsAnyAttachmentPhoto(tweet)
		if photoAttached == true {

			expression.MediaURL = tweet.Entities.Media[mediaIndex].MediaURL

			imagebytes := DownloadImage(tweet.Entities.Media[mediaIndex].MediaURL)
			labels, _ := detectLabels(rekogClient, imagebytes)

			expression.AttachmentLabels = labels

		}

		expression.Create()

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

func GetTweetsFromSearchApi(client *twitter.Client) []twitter.Tweet {

	ie := true
	params := &twitter.SearchTweetParams{
		Query:           "technology AND -filter:retweets AND -filter:replies",
		Lang:            "en",
		IncludeEntities: &ie,
		TweetMode:       "extended",
		ResultType:      "popular",
		Count:           50,
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
		return "nil", err
	}
	var s []string

	if len(result.Labels) > 0 {

		for _, element := range result.Labels {

			s = append(s, *element.Name)

		}

	}

	return strings.Join(s, " "), nil

}

package main

import (
	"context"
	"io"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
	ffmpeg_go "github.com/u2takey/ffmpeg-go"
	"github.com/wader/goutubedl"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("No .env file found")
	}
}

func main() {
	BOT_TOKEN, exists := os.LookupEnv("BOT_TOKEN")
	if !exists {
		log.Fatal("Unable to find BOT_TOKEN in .env file")
	}

	goutubedl.Path = "yt-dlp"

	bot, err := telego.NewBot(
		BOT_TOKEN,
		telego.WithDefaultLogger(false, true),
	)
	if err != nil {
		log.Fatal(err)
	}

	updates, _ := bot.UpdatesViaLongPolling(nil)
	defer bot.StopLongPolling()

	for update := range updates {
		if update.Message == nil {
			continue
		}

		chat_id := telegoutil.ID(update.Message.Chat.ID)
		splittedMessage := strings.Split(update.Message.Text, " ")
		messageUrl := GetUrl(splittedMessage)
		timeStamps := GetTimeStamps(splittedMessage)

		if messageUrl != "" {
			downloadedName, _ := Downloader(bot, messageUrl, chat_id)
			err := Trimmer(downloadedName, timeStamps)

			if err != nil {
				bot.SendMessage(&telego.SendMessageParams{
					ChatID: chat_id,
					Text:   "😔 Unable to trim audio file",
				})
			}

			fileToSend, err := os.Open(downloadedName + ".ogg")

			if err != nil {
				log.Fatal("Unable to open .ogg file")
			}

			bot.SendVoice(&telego.SendVoiceParams{
				ChatID: chat_id,
				Voice: telego.InputFile{
					File: fileToSend,
				},
				Caption: downloadedName,
			})

			fileToSend.Close()
			os.Remove(downloadedName)
			os.Remove(downloadedName + ".ogg")

		} else {
			bot.SendMessage(&telego.SendMessageParams{
				ChatID: chat_id,
				Text:   "😔 Your message does not contain a link.",
			})
		}

	}
}

func GetTimeStamps(data []string) []string {
	output := make([]string, 2)
	timestampsCount := 0
	for _, element := range data {
		if IsTimeStamp(element) {
			output[timestampsCount] = element
			timestampsCount++
		}
		if timestampsCount == 2 {
			return output
		}
	}
	return make([]string, 0)
}

func IsTimeStamp(t string) bool {
	splittedTime := strings.Split(t, ":")
	return len(splittedTime) == 3 && AllIsNumeric(splittedTime)
}

func AllIsNumeric(data []string) bool {
	check1 := regexp.MustCompile(`\d`).MatchString(data[0])
	check2 := regexp.MustCompile(`\d`).MatchString(data[1])
	check3 := regexp.MustCompile(`(?:0|[1-9]\d*)(?:\.\d*)?`).MatchString(data[2])

	return check1 && check2 && check3
}

func IsUrl(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func GetUrl(messageParts []string) string {
	for _, part := range messageParts {
		if IsUrl(part) {
			return part
		}
	}
	return ""
}

func Trimmer(file string, borders []string) error {
	if len(borders) == 2 {
		err := ffmpeg_go.Input(file, ffmpeg_go.KwArgs{
			"ss": borders[0],
		}).Output(file+".ogg", ffmpeg_go.KwArgs{
			"to":       borders[1],
			"loglevel": "quiet",
		}).OverWriteOutput().ErrorToStdOut().Silent(true).Run()
		if err != nil {
			return err
		}
	} else if len(borders) == 0 {
		err := ffmpeg_go.Input(file).Output(file+".ogg", ffmpeg_go.KwArgs{
			"loglevel": "quiet",
		}).OverWriteOutput().ErrorToStdOut().Run()
		if err != nil {
			return err
		}
	}
	return nil
}

func Downloader(b *telego.Bot, link string, chat_id telego.ChatID) (string, error) {
	result, err := goutubedl.New(context.Background(), link, goutubedl.Options{
		DownloadSubtitles: false,
		DownloadThumbnail: true,
		Type:              1,
	})

	if err != nil {
		b.SendMessage(&telego.SendMessageParams{
			ChatID: chat_id,
			Text:   "😔 Sorry, the bot can't recognize this link at the moment.",
		})
		log.Fatal(err)
	}

	if result.Info.Duration > 10*float64(time.Minute) {
		return result.Info.Title, nil
	}

	downloadResult, err := result.Download(context.Background(), "bestaudio")
	if err != nil {
		b.SendMessage(&telego.SendMessageParams{
			ChatID: chat_id,
			Text:   "😔 Sorry, the bot can't download audio from this link at the moment.",
		})
		log.Fatal(err)
	}
	defer downloadResult.Close()

	f, err := os.Create(result.Info.Title)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()
	io.Copy(f, downloadResult)
	return result.Info.Title, nil
}

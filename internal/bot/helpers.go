package bot

import (
	"context"
	"time"

	"github.com/WIZARDISHUNGRY/hls-await/internal/logger"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/pkg/errors"
)

func getLastTweet(c *twitter.Client) (int64, time.Time, error) {
	u, _, err := c.Accounts.VerifyCredentials(&twitter.AccountVerifyParams{
		IncludeEntities: twitter.Bool(true),
	})
	if err != nil {
		return 0, time.Time{}, errors.Wrap(err, "VerifyCredentials")
	}
	tws, _, err := c.Timelines.UserTimeline(&twitter.UserTimelineParams{UserID: u.ID, Count: 1})
	if len(tws) == 0 {
		return 0, time.Time{}, errors.New("no tweets")
	}
	tw := tws[0]
	tm, err := time.Parse(time.RubyDate, tw.CreatedAt)
	if err != nil {
		return 0, time.Time{}, errors.Wrap(err, "time.Parse")
	}
	return tw.ID, tm, nil
}

func (b *Bot) getLastTweetMaybe(ctx context.Context) {
	log := logger.Entry(ctx)
	id, tm, err := getLastTweet(b.client)
	if err != nil {
		log.WithError(err).Warn("getLastTweet")
		return
	}
	log.WithField("age", time.Now().Sub(tm)).Info("found old tweet")
	b.lastID = id
	b.lastPosted = tm
}

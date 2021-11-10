package do

import (
	"context"
	"github.com/Qovery/pleco/pkg/common"
	"github.com/digitalocean/godo"
	log "github.com/sirupsen/logrus"
	"time"
)

type DODB struct {
	ID           string
	Name         string
	CreationDate time.Time
	TTL          int64
	IsProtected  bool
}

func DeleteExpiredDatabases(sessions DOSessions, options DOOptions) {
	expiredDatabases := getExpiredDatabases(sessions.Client, options.TagName, options.Region)

	count, start := common.ElemToDeleteFormattedInfos("expired database", len(expiredDatabases), options.Region)

	log.Debug(count)

	if options.DryRun || len(expiredDatabases) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredDb := range expiredDatabases {
		deleteDB(sessions.Client, expiredDb)
	}
}

func getExpiredDatabases(client *godo.Client, tagName string, region string) []DODB {
	databases := listDatabases(client, tagName, region)

	expiredDbs := []DODB{}
	for _, db := range databases {
		if common.CheckIfExpired(db.CreationDate, db.TTL, "database"+db.Name) && !db.IsProtected {
			expiredDbs = append(expiredDbs, db)
		}
	}

	return expiredDbs
}

func listDatabases(client *godo.Client, tagName string, region string) []DODB {
	result, _, err := client.Databases.List(context.TODO(), &godo.ListOptions{})

	if err != nil {
		log.Errorf("Can't list databases for region %s: %s", region, err.Error())
		return []DODB{}
	}

	databases := []DODB{}
	for _, db := range result {
		essentialTags := common.GetEssentialTags(db.Tags, tagName)
		creationDate, _ := time.Parse(time.RFC3339, db.CreatedAt.Format(time.RFC3339))

		databases = append(databases, DODB{
			ID:           db.ID,
			Name:         db.Name,
			CreationDate: creationDate,
			TTL:          essentialTags.TTL,
			IsProtected:  essentialTags.IsProtected,
		})
	}

	return databases
}

func deleteDB(client *godo.Client, db DODB) {
	_, err := client.Databases.Delete(context.TODO(), db.ID)

	if err != nil {
		log.Errorf("Can't delete database %s: %s", db.Name, err.Error())
	}
}

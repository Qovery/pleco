package gcp

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Qovery/pleco/pkg/common"
	log "github.com/sirupsen/logrus"
)

// VolatileObject reports whether a resource is eligible for cleanup under its
// own policy. Expiration can mean an elapsed TTL or confirmed orphan status.
// IsExpired never deletes resources; callers handle dry-run and deletion.
// Dependencies such as subnets and bucket contents follow their parent's policy.
type VolatileObject interface {
	IsExpired() bool
}

var (
	_ VolatileObject = Bucket{}
	_ VolatileObject = ArtifactRegistryRepository{}
	_ VolatileObject = GKECluster{}
	_ VolatileObject = Job{}
	_ VolatileObject = Network{}
	_ VolatileObject = Router{}
	_ VolatileObject = ServiceAccount{}
	_ VolatileObject = Disk{}
	_ VolatileObject = DeletedServiceAccountBinding{}
	_ VolatileObject = MissingServiceAccountBinding{}
)

func resourceTTL(kind, name, value string) (int64, bool) {
	if strings.TrimSpace(value) == "" {
		log.Infof("No ttl value found, ignoring this %s (`%s`)", kind, name)
		return 0, false
	}
	ttl, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		log.Warnf("ttl value `%s` is not parsable to int64, ignoring this %s (`%s`)", value, kind, name)
		return 0, false
	}
	return ttl, ttl != 0
}

func isExpiredFromCreationTime(kind, name, ttlValue string, creationTime time.Time) bool {
	ttl, ok := resourceTTL(kind, name, ttlValue)
	return ok && !time.Now().UTC().Before(creationTime.UTC().Add(time.Second*time.Duration(ttl)))
}

func isExpiredFromUnixCreation(kind, name, ttlValue, creationValue string, allowEpoch bool) bool {
	ttl, ok := resourceTTL(kind, name, ttlValue)
	if !ok {
		return false
	}
	if creationValue == "" {
		log.Infof("No creation_date value found, ignoring this %s (`%s`)", kind, name)
		return false
	}
	creationUnix, err := strconv.ParseInt(creationValue, 10, 64)
	if err != nil {
		log.Warnf("creation_date value `%s` is not parsable to int64, ignoring this %s (`%s`)", creationValue, kind, name)
		return false
	}
	if creationUnix == 0 && !allowEpoch {
		return false
	}
	creationTime := time.Unix(creationUnix, 0).UTC()
	return !time.Now().UTC().Before(creationTime.Add(time.Second * time.Duration(ttl)))
}

func isExpiredFromDescription(kind, name, description string) bool {
	var tags common.ResourceTags
	if err := json.Unmarshal([]byte(description), &tags); err != nil {
		log.Infof("No resource tags found in description field, ignoring this %s (`%s`)", kind, name)
		return false
	}
	var ttlValue, creationValue string
	if tags.TTL != nil {
		ttlValue = tags.TTL.String()
	}
	if tags.CreationUnixTimestamp != nil {
		creationValue = tags.CreationUnixTimestamp.String()
	}
	return isExpiredFromUnixCreation(kind, name, ttlValue, creationValue, false)
}

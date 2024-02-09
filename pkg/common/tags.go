package common

import "encoding/json"

type ResourceTags struct {
	TTL                   *json.Number `json:"ttl"`
	CreationUnixTimestamp *json.Number `json:"creation_date"`
}

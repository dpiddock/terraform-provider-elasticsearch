package es

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/structure"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"

	elastic7 "github.com/olivere/elastic/v7"
	elastic6 "gopkg.in/olivere/elastic.v6"
)

var xPackWatchSchema = map[string]*schema.Schema{
	"watch_id": {
		Type:     schema.TypeString,
		Required: true,
		ForceNew: true,
	},
	"body": {
		Type:             schema.TypeString,
		Required:         true,
		ValidateFunc:     validation.StringIsJSON,
		DiffSuppressFunc: suppressEquivalentJson,
		StateFunc: func(v interface{}) string {
			json, _ := structure.NormalizeJsonString(v)
			return json
		},
	},
	"active": {
		Type:        schema.TypeBool,
		Required:    false,
		Optional:    true,
		Default:     true,
		Description: "Boolean to activate the xpack watcher, defaults `true`",
	},
}

func resourceElasticsearchDeprecatedWatch() *schema.Resource {
	return &schema.Resource{
		Create: resourceElasticsearchWatchCreate,
		Read:   resourceElasticsearchWatchRead,
		Update: resourceElasticsearchWatchUpdate,
		Delete: resourceElasticsearchWatchDelete,
		Schema: xPackWatchSchema,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		DeprecationMessage: "elasticsearch_watch is deprecated, please use elasticsearch_xpack_watch resource instead.",
	}
}

func resourceElasticsearchXpackWatch() *schema.Resource {
	return &schema.Resource{
		Create: resourceElasticsearchWatchCreate,
		Read:   resourceElasticsearchWatchRead,
		Update: resourceElasticsearchWatchUpdate,
		Delete: resourceElasticsearchWatchDelete,
		Schema: xPackWatchSchema,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
	}
}

func resourceElasticsearchWatchCreate(d *schema.ResourceData, m interface{}) error {
	// Determine whether the watch already exists, otherwise the API will
	// override an existing watch with the name.
	watchID := d.Get("watch_id").(string)
	_, err := resourceElasticsearchGetWatch(watchID, m)

	if err == nil {
		log.Printf("[INFO] watch exists: %+v", err)
		return fmt.Errorf("watch already exists with ID: %v", watchID)
	} else if err != nil && !elastic6.IsNotFound(err) && !elastic7.IsNotFound(err) {
		return err
	}

	watchID, err = resourceElasticsearchPutWatch(d, m)

	if err != nil {
		log.Printf("[INFO] Failed to put watch: %+v", err)
		return err
	}

	d.SetId(watchID)
	log.Printf("[INFO] Object ID: %s", d.Id())

	return resourceElasticsearchWatchRead(d, m)
}

func resourceElasticsearchWatchRead(d *schema.ResourceData, m interface{}) error {
	res, err := resourceElasticsearchGetWatch(d.Id(), m)

	if elastic6.IsNotFound(err) || elastic7.IsNotFound(err) {
		log.Printf("[WARN] Watch (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return err
	}

	var watch []byte
	status := false

	esClient, err := getClient(m.(*ProviderConf))
	if err != nil {
		return err
	}
	switch esClient.(type) {
	case *elastic7.Client:
		watchResponse := res.(*elastic7.XPackWatcherGetWatchResponse)
		watch, err = json.Marshal(watchResponse.Watch)
		status = watchResponse.Status.State.Active
	case *elastic6.Client:
		watchResponse := res.(*elastic6.XPackWatcherGetWatchResponse)
		watch, err = json.Marshal(watchResponse.Watch)
		status = watchResponse.Status.State.Active
	}

	if err != nil {
		return err
	}

	ds := &resourceDataSetter{d: d}
	ds.set("body", string(watch))
	ds.set("watch_id", d.Id())
	ds.set("active", status)

	return ds.err
}

func resourceElasticsearchWatchUpdate(d *schema.ResourceData, m interface{}) error {
	_, err := resourceElasticsearchPutWatch(d, m)

	if err != nil {
		return err
	}

	return resourceElasticsearchWatchRead(d, m)
}

func resourceElasticsearchWatchDelete(d *schema.ResourceData, m interface{}) error {
	var err error
	esClient, err := getClient(m.(*ProviderConf))
	if err != nil {
		return err
	}
	switch client := esClient.(type) {
	case *elastic7.Client:
		_, err = client.XPackWatchDelete(d.Id()).Do(context.TODO())
	case *elastic6.Client:
		_, err = client.XPackWatchDelete(d.Id()).Do(context.TODO())
	default:
		err = errors.New("watch resource not implemented prior to Elastic v6")
	}

	return err
}

func resourceElasticsearchGetWatch(watchID string, m interface{}) (interface{}, error) {
	var res interface{}
	var err error
	esClient, err := getClient(m.(*ProviderConf))
	if err != nil {
		return "", err
	}
	switch client := esClient.(type) {
	case *elastic7.Client:
		res, err = client.XPackWatchGet(watchID).Do(context.TODO())
	case *elastic6.Client:
		res, err = client.XPackWatchGet(watchID).Do(context.TODO())
	default:
		err = errors.New("watch resource not implemented prior to Elastic v6")
	}

	return res, err
}

func resourceElasticsearchPutWatch(d *schema.ResourceData, m interface{}) (string, error) {
	watchID := d.Get("watch_id").(string)
	watchJSON := d.Get("body").(string)
	isActive := d.Get("active").(bool)

	var err error
	esClient, err := getClient(m.(*ProviderConf))
	if err != nil {
		return "", err
	}
	switch client := esClient.(type) {
	case *elastic7.Client:
		_, err = client.XPackWatchPut(watchID).
			Body(watchJSON).
			Do(context.TODO())
	case *elastic6.Client:
		_, err = client.XPackWatchPut(watchID).
			Body(watchJSON).
			Do(context.TODO())
	default:
		err = errors.New("watch resource not implemented prior to Elastic v6")
	}

	if err != nil {
		return "", err
	}

	_, err = activateWatcher(esClient, watchID, isActive)

	if err != nil {
		return "", err
	}

	return watchID, nil
}

// turn on or off the watcher
func activateWatcher(esClient interface{}, watchID string, isActive bool) (string, error) {
	var err error
	switch client := esClient.(type) {
	case *elastic7.Client:
		if isActive {
			_, err = client.XPackWatchActivate(watchID).Do(context.TODO())
		} else {
			_, err = client.XPackWatchDeactivate(watchID).Do(context.TODO())
		}
	case *elastic6.Client:
		if isActive {
			_, err = client.XPackWatchActivate(watchID).Do(context.TODO())
		} else {
			_, err = client.XPackWatchDeactivate(watchID).Do(context.TODO())
		}
	default:
		err = errors.New("watch resource not implemented prior to Elastic v6")
	}

	if err != nil {
		return "", err
	}

	return "", err
}

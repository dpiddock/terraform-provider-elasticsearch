package es

import (
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	elastic7 "github.com/olivere/elastic/v7"
	elastic5 "gopkg.in/olivere/elastic.v5"
	elastic6 "gopkg.in/olivere/elastic.v6"
)

func dataSourceElasticsearchHost() *schema.Resource {
	return &schema.Resource{
		Description: "`elasticsearch_host` can be used to retrieve the host URL for the provider's current elasticsearch cluster.",
		Read:        dataSourceElasticsearchHostRead,

		Schema: map[string]*schema.Schema{
			"active": {
				Type:        schema.TypeBool,
				Required:    true,
				Description: "should be set to `true`",
			},
			"url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "the url of the active elasticsearch cluster",
			},
		},
	}
}

func dataSourceElasticsearchHostRead(d *schema.ResourceData, m interface{}) error {

	// The upstream elastic client does not export the property for the urls
	// it's using. Presumably the URLS would be available where the client is
	// intantiated, but in terraform, that's not always practicable.
	var err error
	esClient, err := getClient(m.(*ProviderConf))
	if err != nil {
		return err
	}
	switch client := esClient.(type) {
	case *elastic7.Client:
		urls := reflect.ValueOf(client).Elem().FieldByName("urls")
		if urls.Len() > 0 {
			d.SetId(urls.Index(0).String())
		}
	case *elastic6.Client:
		urls := reflect.ValueOf(client).Elem().FieldByName("urls")
		if urls.Len() > 0 {
			d.SetId(urls.Index(0).String())
		}
	default:
		client = esClient.(*elastic5.Client)

		urls := reflect.ValueOf(client).Elem().FieldByName("urls")
		if urls.Len() > 0 {
			d.SetId(urls.Index(0).String())
		}
	}

	return err
}

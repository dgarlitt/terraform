package aws

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

func resourceAwsCloudWatchMetricFilter() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsCloudWatchMetricFilterCreate,
		Read:   resourceAwsCloudWatchMetricFilterRead,
		Delete: resourceAwsCloudWatchMetricFilterDelete,

		Schema: map[string]*schema.Schema{
			"filter_name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: func(v interface{}, k string) (ws []string, errors []error) {
					validateStringLengthAndPattern(v.(string), k, 512, `[^:*]*`, errors)
					return
				},
			},
			"filter_pattern": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: func(v interface{}, k string) (ws []string, errors []error) {
					validateStringLengthAndPattern(v.(string), k, 512, ``, errors)
					return
				},
			},
			"log_group_name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: func(v interface{}, k string) (ws []string, errors []error) {
					validateStringLengthAndPattern(v.(string), k, 512, `[\.\-_/#A-Za-z0-9]+`, errors)
					return
				},
			},
			"metric_transformations": &schema.Schema{
				Type:     schema.TypeSet,
				Required: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"metric_name": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: func(v interface{}, k string) (ws []string, errors []error) {
								validateStringLengthAndPattern(v.(string), k, 255, `[^:*$]*`, errors)
								return
							},
						},
						"metric_namespace": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: func(v interface{}, k string) (ws []string, errors []error) {
								validateStringLengthAndPattern(v.(string), k, 255, `[^:*$]*`, errors)
								return
							},
						},
						"metric_value": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: func(v interface{}, k string) (ws []string, errors []error) {
								validateStringLengthAndPattern(v.(string), k, 100, ``, errors)
								return
							},
						},
					},
				},
			},
		},
	}
}

func resourceAwsCloudWatchMetricFilterCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).cloudwatchlogsconn
	params := cloudwatchlogs.PutMetricFilterInput{
		FilterName:    aws.String(d.Get("filter_name").(string)),
		FilterPattern: aws.String(d.Get("filter_pattern").(string)),
		LogGroupName:  aws.String(d.Get("log_group_name").(string)),
	}

	if attr, ok := d.GetOk("metric_transformations"); ok {
		metricFilters := buildMetricTransformations(attr.(*schema.Set).List())
		params.MetricTransformations = metricFilters
	}

	log.Printf("[DEBUG] Error creating CloudWatch Metric Filter: %#v", d)
	_, err := conn.PutMetricFilter(&params)
	if err != nil {
		return fmt.Errorf("CloudWatch Metric Filter creation failed: %s", err)
	}

	d.SetId(fmt.Sprintf("%s:%s", *params.FilterName, *params.LogGroupName))
	log.Println("[INFO] CloudWatch Metric Filter created")

	return resourceAwsCloudWatchMetricFilterRead(d, meta)
}

func resourceAwsCloudWatchMetricFilterRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).cloudwatchlogsconn
	fn, lgn := parseCloudWatchMetricFilterID(d.Id())

	log.Printf("[DEBUG] Reading CloudWatch Metric Filter: %s", d.Get("filter_name"))

	mf, err := lookupCloudWatchMetricFilter(conn, fn, lgn, nil)
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Found CloudWatch Metric Filter: %#v", *mf)

	d.Set("filter_name", *mf.FilterName)
	d.Set("filter_pattern", *mf.FilterPattern)
	d.Set("log_group_name", aws.String(lgn))
	if err := d.Set("metric_transformations", getMetricTransformationsAsMapSlice(mf.MetricTransformations)); err != nil {
		return err
	}

	return nil
}

func resourceAwsCloudWatchMetricFilterDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).cloudwatchlogsconn
	filterName, logGroupName := parseCloudWatchMetricFilterID(d.Id())

	log.Printf("[INFO] Deleting Cloudwatch Metric Filter: %s", d.Id())

	_, err := conn.DeleteMetricFilter(&cloudwatchlogs.DeleteMetricFilterInput{
		FilterName:   aws.String(filterName),
		LogGroupName: aws.String(logGroupName),
	})
	if err != nil {
		return fmt.Errorf("Error deleting CloudWatch Metric Filter: %s", err)
	}

	log.Println("[INFO] CloudWatch Metric Filter deleted")

	d.SetId("")

	return nil
}

func validateStringLengthAndPattern(v string, k string, max int, pattern string, errors []error) {
	if len(v) > max {
		errors = append(errors, fmt.Errorf(
			"%q cannot be longer than %d characters", k, max))
	}

	if len(pattern) > 0 {
		r, err := regexp.Compile(pattern)

		if err == nil && r.MatchString(v) == false {
			errors = append(errors, fmt.Errorf(
				"%q must match the pattern %q", k, pattern))
		}
	}
}

func lookupCloudWatchMetricFilter(conn *cloudwatchlogs.CloudWatchLogs,
	filterName string, logGroupName string, nextToken *string) (*cloudwatchlogs.MetricFilter, error) {
	input := &cloudwatchlogs.DescribeMetricFiltersInput{
		FilterNamePrefix: aws.String(filterName),
		LogGroupName:     aws.String(logGroupName),
		NextToken:        nextToken,
	}

	resp, err := conn.DescribeMetricFilters(input)
	if err != nil {
		return nil, err
	}

	for _, mf := range resp.MetricFilters {
		if *mf.FilterName == filterName {
			return mf, nil
		}
	}

	if resp.NextToken != nil {
		return lookupCloudWatchMetricFilter(conn, filterName, logGroupName, resp.NextToken)
	}

	return nil, fmt.Errorf("CloudWatch Metric Filter %q for Log Group %q not found", filterName, logGroupName)
}

func buildMetricTransformations(configured []interface{}) []*cloudwatchlogs.MetricTransformation {
	var filters []*cloudwatchlogs.MetricTransformation
	for _, raw := range configured {
		var filter cloudwatchlogs.MetricTransformation
		m := raw.(map[string]interface{})

		filter.MetricName = aws.String(m["metric_name"].(string))
		filter.MetricNamespace = aws.String(m["metric_namespace"].(string))
		filter.MetricValue = aws.String(m["metric_value"].(string))

		filters = append(filters, &filter)
	}

	return filters
}

func getMetricTransformationsAsMapSlice(list []*cloudwatchlogs.MetricTransformation) []map[string]string {
	result := make([]map[string]string, 0, len(list))
	for _, mt := range list {
		item := make(map[string]string)
		item["metric_name"] = *mt.MetricName
		item["metric_namespace"] = *mt.MetricNamespace
		item["metric_value"] = *mt.MetricValue

		result = append(result, item)
	}

	return result
}

func parseCloudWatchMetricFilterID(id string) (filterName, logGroupName string) {
	parts := strings.SplitN(id, ":", 2)
	filterName = parts[0]
	logGroupName = parts[1]
	return
}

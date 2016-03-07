package aws

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
)

func TestAccAWSCloudWatchMetricFilter_basic(t *testing.T) {
	var mf cloudwatchlogs.MetricFilter

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSCloudWatchMetricFilterDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSCloudWatchMetricFilterConfig,
				Check: resource.ComposeTestCheckFunc(
					testMetricFilterRequiredResourcesExist(
						"aws_cloudwatch_log_group.bazqux",
						"aws_cloudwatch_metric_filter.foobar", &mf),
					resource.TestCheckResourceAttr("aws_cloudwatch_metric_filter.foobar", "filter_name", "foo-bar-filter"),
					resource.TestCheckResourceAttr("aws_cloudwatch_metric_filter.foobar", "filter_pattern", "{ ($.foo = \"bar\") }"),
					resource.TestCheckResourceAttr("aws_cloudwatch_metric_filter.foobar", "log_group_name", "foo-bar"),
					resource.TestCheckResourceAttr("aws_cloudwatch_metric_filter.foobar", "log_group_name", "foo-bar"),
				),
			},
		},
	})
}

func testAccCheckAWSCloudWatchMetricFilterDestroy(s *terraform.State) error {
	conn := testAccProvider.Meta().(*AWSClient).cloudwatchlogsconn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_cloudwatch_metric_filter" {
			continue
		}

		filterName, logGroupName := parseCloudWatchMetricFilterID(rs.Primary.ID)

		_, err := lookupCloudWatchMetricFilter(conn, filterName, logGroupName, nil)
		if err == nil {
			return fmt.Errorf("MetricFilter Still Exists: %s", rs.Primary.ID)
		}
	}

	return nil
}

func testMetricFilterRequiredResourcesExist(
	logGroupResource string,
	metricFilterResource string,
	metricFilter *cloudwatchlogs.MetricFilter) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs1, ok := s.RootModule().Resources[logGroupResource]
		if !ok {
			return fmt.Errorf("Not found: %s", logGroupResource)
		}

		if rs1.Primary.ID == "" {
			return fmt.Errorf("No ID set: %s", logGroupResource)
		}

		rs2, ok := s.RootModule().Resources[metricFilterResource]
		if !ok {
			return fmt.Errorf("Not found: %s", metricFilterResource)
		}

		conn := testAccProvider.Meta().(*AWSClient).cloudwatchlogsconn
		filterName, logGroupName := parseCloudWatchMetricFilterID(rs2.Primary.ID)
		params := cloudwatchlogs.DescribeMetricFiltersInput{
			FilterNamePrefix: aws.String(filterName),
			LogGroupName:     aws.String(logGroupName),
		}
		resp, err := conn.DescribeMetricFilters(&params)
		if err != nil {
			return err
		}
		if len(resp.MetricFilters) == 0 {
			return fmt.Errorf("Metric Filter not found")
		}
		*metricFilter = *resp.MetricFilters[0]

		return nil
	}
}

var testAccAWSCloudWatchMetricFilterConfig = `
resource "aws_cloudwatch_log_group" "bazqux" {
    name = "foo-bar"
}

resource "aws_cloudwatch_metric_filter" "foobar" {
	filter_name = "foo-bar-filter"
	filter_pattern = "{ ($.foo = \"bar\") }"
	log_group_name = "${aws_cloudwatch_log_group.bazqux.name}"
	metric_transformations = {
		metric_name = "foo-bar-metric"
		metric_namespace = "foo/bar"
		metric_value = "1"
	}
}
`

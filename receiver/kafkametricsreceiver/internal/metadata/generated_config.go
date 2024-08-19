// Code generated by mdatagen. DO NOT EDIT.

package metadata

import (
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/filter"
)

// MetricConfig provides common config for a particular metric.
type MetricConfig struct {
	Enabled bool `mapstructure:"enabled"`

	enabledSetByUser bool
}

func (ms *MetricConfig) Unmarshal(parser *confmap.Conf) error {
	if parser == nil {
		return nil
	}
	err := parser.Unmarshal(ms)
	if err != nil {
		return err
	}
	ms.enabledSetByUser = parser.IsSet("enabled")
	return nil
}

// MetricsConfig provides config for kafkametrics metrics.
type MetricsConfig struct {
	KafkaBrokerLogRetentionPeriod MetricConfig `mapstructure:"kafka.broker.log_retention_period"`
	KafkaBrokers                  MetricConfig `mapstructure:"kafka.brokers"`
	KafkaConsumerGroupLag         MetricConfig `mapstructure:"kafka.consumer_group.lag"`
	KafkaConsumerGroupLagSum      MetricConfig `mapstructure:"kafka.consumer_group.lag_sum"`
	KafkaConsumerGroupMembers     MetricConfig `mapstructure:"kafka.consumer_group.members"`
	KafkaConsumerGroupOffset      MetricConfig `mapstructure:"kafka.consumer_group.offset"`
	KafkaConsumerGroupOffsetSum   MetricConfig `mapstructure:"kafka.consumer_group.offset_sum"`
	KafkaPartitionCurrentOffset   MetricConfig `mapstructure:"kafka.partition.current_offset"`
	KafkaPartitionOldestOffset    MetricConfig `mapstructure:"kafka.partition.oldest_offset"`
	KafkaPartitionReplicas        MetricConfig `mapstructure:"kafka.partition.replicas"`
	KafkaPartitionReplicasInSync  MetricConfig `mapstructure:"kafka.partition.replicas_in_sync"`
	KafkaTopicLogRetentionPeriod  MetricConfig `mapstructure:"kafka.topic.log_retention_period"`
	KafkaTopicLogRetentionSize    MetricConfig `mapstructure:"kafka.topic.log_retention_size"`
	KafkaTopicMinInsyncReplicas   MetricConfig `mapstructure:"kafka.topic.min_insync_replicas"`
	KafkaTopicPartitions          MetricConfig `mapstructure:"kafka.topic.partitions"`
	KafkaTopicReplicationFactor   MetricConfig `mapstructure:"kafka.topic.replication_factor"`
}

func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		KafkaBrokerLogRetentionPeriod: MetricConfig{
			Enabled: false,
		},
		KafkaBrokers: MetricConfig{
			Enabled: true,
		},
		KafkaConsumerGroupLag: MetricConfig{
			Enabled: true,
		},
		KafkaConsumerGroupLagSum: MetricConfig{
			Enabled: true,
		},
		KafkaConsumerGroupMembers: MetricConfig{
			Enabled: true,
		},
		KafkaConsumerGroupOffset: MetricConfig{
			Enabled: true,
		},
		KafkaConsumerGroupOffsetSum: MetricConfig{
			Enabled: true,
		},
		KafkaPartitionCurrentOffset: MetricConfig{
			Enabled: true,
		},
		KafkaPartitionOldestOffset: MetricConfig{
			Enabled: true,
		},
		KafkaPartitionReplicas: MetricConfig{
			Enabled: true,
		},
		KafkaPartitionReplicasInSync: MetricConfig{
			Enabled: true,
		},
		KafkaTopicLogRetentionPeriod: MetricConfig{
			Enabled: false,
		},
		KafkaTopicLogRetentionSize: MetricConfig{
			Enabled: false,
		},
		KafkaTopicMinInsyncReplicas: MetricConfig{
			Enabled: false,
		},
		KafkaTopicPartitions: MetricConfig{
			Enabled: true,
		},
		KafkaTopicReplicationFactor: MetricConfig{
			Enabled: false,
		},
	}
}

// ResourceAttributeConfig provides common config for a particular resource attribute.
type ResourceAttributeConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// Experimental: MetricsInclude defines a list of filters for attribute values.
	// If the list is not empty, only metrics with matching resource attribute values will be emitted.
	MetricsInclude []filter.Config `mapstructure:"metrics_include"`
	// Experimental: MetricsExclude defines a list of filters for attribute values.
	// If the list is not empty, metrics with matching resource attribute values will not be emitted.
	// MetricsInclude has higher priority than MetricsExclude.
	MetricsExclude []filter.Config `mapstructure:"metrics_exclude"`

	enabledSetByUser bool
}

func (rac *ResourceAttributeConfig) Unmarshal(parser *confmap.Conf) error {
	if parser == nil {
		return nil
	}
	err := parser.Unmarshal(rac)
	if err != nil {
		return err
	}
	rac.enabledSetByUser = parser.IsSet("enabled")
	return nil
}

// ResourceAttributesConfig provides config for kafkametrics resource attributes.
type ResourceAttributesConfig struct {
	KafkaClusterAlias ResourceAttributeConfig `mapstructure:"kafka.cluster.alias"`
}

func DefaultResourceAttributesConfig() ResourceAttributesConfig {
	return ResourceAttributesConfig{
		KafkaClusterAlias: ResourceAttributeConfig{
			Enabled: false,
		},
	}
}

// MetricsBuilderConfig is a configuration for kafkametrics metrics builder.
type MetricsBuilderConfig struct {
	Metrics            MetricsConfig            `mapstructure:"metrics"`
	ResourceAttributes ResourceAttributesConfig `mapstructure:"resource_attributes"`
}

func DefaultMetricsBuilderConfig() MetricsBuilderConfig {
	return MetricsBuilderConfig{
		Metrics:            DefaultMetricsConfig(),
		ResourceAttributes: DefaultResourceAttributesConfig(),
	}
}

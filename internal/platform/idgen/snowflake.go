package idgen

import (
	"fmt"
	"hash/fnv"
	"os"
	"strings"
	"sync"

	"github.com/bwmarrin/snowflake"
)

var (
	messageNodeOnce sync.Once
	messageNode     *snowflake.Node
	messageNodeErr  error
)

func MessageID() string {
	node, err := messageSnowflakeNode()
	if err != nil {
		panic(fmt.Errorf("init message snowflake node: %w", err))
	}

	return "M" + node.Generate().String()
}

func messageSnowflakeNode() (*snowflake.Node, error) {
	messageNodeOnce.Do(func() {
		messageNode, messageNodeErr = snowflake.NewNode(resolveSnowflakeNodeNumber())
	})

	return messageNode, messageNodeErr
}

// 消息 ID 生成需要在多节点部署下稳定区分不同 worker。
// 这里优先使用部署时注入的 node_id 环境变量；未显式配置时再回退到 hostname，
// 这样 docker-compose 多节点下无需再额外维护一套消息 ID worker 编号，
// 单测环境也不会被配置文件加载顺序卡住。
func resolveSnowflakeNodeNumber() int64 {
	candidates := []string{
		strings.TrimSpace(os.Getenv("DIPOLE_PRESENCE_NODE_ID")),
		strings.TrimSpace(os.Getenv("DIPOLE_KAFKA_CLIENT_ID")),
		strings.TrimSpace(os.Getenv("HOSTNAME")),
		"dipole",
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}

		hasher := fnv.New32a()
		_, _ = hasher.Write([]byte(candidate))
		return int64(hasher.Sum32() % 1024)
	}

	return 0
}

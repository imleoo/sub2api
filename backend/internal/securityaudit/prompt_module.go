package securityaudit

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewPostgreSQLRepository,
	wire.Bind(new(JobRepository), new(*PostgreSQLRepository)),
	wire.Bind(new(EventRepository), new(*PostgreSQLRepository)),
	NewRedisPayloadStore,
	wire.Bind(new(PayloadStore), new(*RedisPayloadStore)),
	NewOpenAICompatibleScanner,
	wire.Bind(new(PromptScanner), new(*OpenAICompatibleScanner)),
	NewAtomicMetrics,
	wire.Bind(new(Metrics), new(*AtomicMetrics)),
	NewConfigManager,
	wire.Bind(new(ConfigStore), new(*ConfigManager)),
	NewPromptService,
	wire.Bind(new(PromptEngine), new(*PromptService)),
	// fork: 显式绑定 PromptAdminService，供离线 `go generate ./cmd/server`
	// 重新生成 wire_gen.go 时解析 NewPromptAdminHandler 的接口入参（上游把
	// wire_gen.go 提交进仓库，Go 编译隐式满足接口故未显式声明此绑定）。
	wire.Bind(new(PromptAdminService), new(*PromptService)),
	NewLegacyModerationAdapter,
	NewCoordinator,
	NewPromptAdminHandler,
)

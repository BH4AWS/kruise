package utilasi

import (
	"strings"
)

// 配置文件中run_type的取值，只能填以下三个值中的一个
const (
	RunTypeTest   = "test"  // 测试环境
	RunTypeDaily  = "daily" // 日常环境
	RunTypeProd   = "prod"  // 生产环境
	RunTypeSuffix = "_dadi" // uc使用直接在image后面增加后缀的方法
)

// 1.ref是否符合 DADI image命名规则，以防止转换机把正常的image给覆盖掉.
// 2.防止向normal_image插入记录时，将DADI image插入，造成转换错误
func IsDadiImageName(ref string) bool {
	preStr := "/dadi"
	specStr := "/dadi_"
	flags := [4]string{preStr + RunTypeTest + specStr,
		preStr + RunTypeDaily + specStr,
		preStr + RunTypeProd + specStr,
		//为了兼容老旧的形如reg.docker.alibaba-inc.com/vrbd/dadi_daditest_alios:7u2的dadi ref name
		"/vrbd/dadi_"}

	return (strings.Contains(ref, flags[0]) ||
		strings.Contains(ref, flags[1]) ||
		strings.Contains(ref, flags[2]) ||
		strings.Contains(ref, flags[3]))
}

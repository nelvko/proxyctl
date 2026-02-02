package preflight

// import (
// 	"errors"
// 	"fmt"
// 	"os"
// 	"slices"

// 	"github.com/charmbracelet/huh"
// 	. "github.com/nelvko/proxyctl/log"
// 	"github.com/spf13/cobra"
// 	"github.com/spf13/viper"
// )

// // func CheckRequiredCommands() {
// // 	required := []string{"xz", "pgrep", "curl", "tar", "unzip", "abc", "wgett"}
// // 	missing := []string{}
// // 	for _, v := range required {
// // 		_, err := exec.LookPath(v)
// // 		if err != nil {
// // 			missing = append(missing, v)
// // 		}
// // 	}
// // 	if len(missing) > 0 {
// // 		ErrorQuit("请先安装以下命令：" + strings.Join(missing, ", "))
// // 	}
// // }


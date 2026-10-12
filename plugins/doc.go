// Package plugins 汇集 kernel 宿主的可装载插件。
//
// 命名与能力约定（观察器类插件据此枚举实例）：
//   - 网络插件：Name = "network/"+alias，Kind = "network"，
//     提供能力 vnet.Network[alias]
//   - 隧道插件：Name = "tunnel/"+隧道名，Kind = "tunnel"，
//     提供能力 *vservice.Service[隧道名]
//   - 观察器插件：Name = "api" 等，Kind = "observer"
package plugins

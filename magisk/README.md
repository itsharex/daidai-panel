# 呆呆面板 Magisk 面具模块

本模块允许在 Android 设备上通过 Magisk 面具来安装和运行呆呆面板。

## 功能

- 自动在 Android 设备上部署呆呆面板
- 支持 ARM64 架构
- 开机自启动服务
- 数据存储在 /data/adb/daidai-panel 目录

## 系统要求

- Android 9.0 (API 28) 或更高版本
- 已安装 Magisk v24.0+
- Root 权限

## 安装

1. 在 Magisk Manager 中点击左侧菜单的"模块"
2. 点击"从本地安装"或"安装从存储"
3. 选择本模块的 zip 文件
4. 安装完成后重启系统

## 配置

配置文件位于: `/data/adb/daidai-panel/config.yaml`

默认配置:
- 端口: 5700
- 前端文件: `/data/adb/daidai-panel/web`

## 卸载

在 Magisk Manager 的模块列表中找到"呆呆面板"并点击删除，然后重启即可。

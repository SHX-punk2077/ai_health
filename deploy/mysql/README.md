# 为什么这里没有 init.sql

建表与种子数据统一由 `/migrations` 下的 golang-migrate 脚本管理，
避免出现「docker 初始化脚本」和「版本迁移」两套 schema 来源不一致。

首次启动顺序：mysql → migrate（一次性任务）→ api。

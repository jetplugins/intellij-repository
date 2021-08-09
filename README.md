# intellij-repository
自定义intellij存储库，支持自动生成plugins.xml文件.

## 使用
1. 安装: `go get github.com/Jetplugins/intellij-repository`
2. 将下载的插件放置到运行目录 `jar/zip`文件
3. 启动: `intellij-repository`, 访问：`http://127.0.0.1`

提示：插件变更，需要重新启动服务.

## IDE配置
- 1.配置插件仓库: `File -> Settings -> Plugins -> 设置 -> Mange Plugin Repsitories`
- 2.搜索需要按照的插件例如: `WxbYapiUpload`

## 启动参数
```shell
-p <port>             # 指定服务监听端口，默认80
-d <work_dir>         # 指定文件服务目录，默认当前目录
-df <default_file>    # 指定首页请求`/`默认文件，指定后不再自动生成plugins.xml
-domain <your.domin>  # 指定服务域名, 生成文件下载地址需要, 默认值: http://{host}
```
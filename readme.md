# tmhelper：跨平台类 expect 命令

tmhelper 命令:
- 支持 linux 和 windows (cmd，powershell，git bash...)
- 支持交互式和非交互式脚本环境
- 内嵌 javascript 引擎(完整支持 ECMAScript 5.1)
- 支持加解密文本，增加安全性

go 库文档：[tmhelper 库]

# 功能速览
### 1.命令参数
```shell
tmhelper -h           
Usage of tmhelper:
  -c string
        code, 运行tmhelper js代码
  -e string
        encrypt, 加密文本
  -d string
        decrypt, 解密文本        
  -f string
        file, 运行tmhelper js文件

```
当没有任何参数时，tmhelper 命令将从标准输入读取js代码。

### 2.多种执行方式示例

以下4个命令完全等效：登录ssh后停留在交互式终端
```shell

# 1.指定js文件路径
tmhelper -f ssh.js

# 2.重定向文件到命令的标准输入 
tmhelper < ssh.js

# 3.在参数里写js代码
tmhelper -c 'tmh.Exec(["ssh", "xr@127.0.0.1"]);tmh.Matchs([["yes/no", "yes\n", "C"],["password", "123456\n"]]);tmh.Matchs([["$", "cd /data/git/\n"]]);tmh.Term();'

# 4.shell中编写js代码，然后导入到tmhelper命令的标准输入
tmhelper <<EOF
tmh.CptKey(tmh.Pwd("crypto key"),"encrypted_123_for_check");
tmh.Exec(["ssh", "xr@127.0.0.1"]); // 运行命令

// 执行多个匹配，默认命中任意一个就返回
tmh.Matchs([
    ["yes/no", "yes\n", "C"],       // "C"(continue)标志表示命中后不退出，继续匹配
    ["password", tmh.Dec("encrypted_base64_password")+"\n"],
]);
tmh.Matchs([["$", "cd /data/git/\n"]]); // 登录后打开指定目录

tmh.Term(); // 停留在交互式终端，若要结束则调用 tmh.Exit()
EOF

```
 `ssh.js` 文件内容为：
```js
// 自动登录ssh，并停留在交互式shell

tmh.Exec(["ssh", "xr@127.0.0.1"]); // 运行命令

// 执行多个匹配，默认命中任意一个就返回
tmh.Matchs([
    ["yes/no", "yes\n", "C"],       // "C"(continue)标志表示命中后不退出，继续匹配
    ["password", "123456\n"],
]);
tmh.Matchs([["$", "cd /data/git/\n"]]); // 登录后打开指定目录

tmh.Term(); // 停留在交互式终端，若要结束则调用 tmh.Exit()
```

### 3.加密文本
```shell
# 加密登录密码
export TMHCPTKEY=12345
tmhelper -e login.password
# output: encrypted base64 text
# in js tmh.Dec(output.encrypted) to decrypt
```


### 4.解密文本
```shell
# 解密
export TMHCPTKEY=12345
tmhelper -d encrypted.password
# output: plain text
```
# 安装


# 使用手册


# 感谢与依赖库
- xexpect的go实现： https://github.com/zh-five/xexpect
- 跨平台pty的go实现： https://github.com/iyzyi/aiopty
- 纯go实现的js引擎：  https://github.com/dop251/goja
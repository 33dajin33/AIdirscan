# AIdirscan
让AI帮着写的目录扫描工具~

## 参数:
  -f string
    	包含多个URL的文件路径，每行一个URL
  -h	显示帮助信息
  -i string
    	要忽略的响应码，用逗号分隔 (例如: 404,503)
  -s string
    	要显示的响应码，用逗号分隔 (例如: 200,301,403,404)
  -t int
    	并发线程数 (默认: 10) (default 10)
  -u string
    	目标URL (例如: http://example.com)

## 说明:
  扫描路径从 ./dicc/dicc.txt 文件中读取，每行一个路径

## 示例:
  ./aiscan -u http://example.com -s 200,301,403 -i 404,503 -t 20
  ./aiscan -f urls.txt -s 200,301,403 -i 404 -t 50


-- pcall(os.execute, "git clone git@github.com:kompasim/my-build-tools.git ./my-build-tools")
package.path = package.path .. ";./my-build-tools/?.lua"
local HtmlBuilder = require("html_builder")
local CodeBuilder = require("./code_builder")

-- windows, darwin, linux
-- $Env:GOOS=""
-- $Env:GOARCH="amd64"
-- os.execute("rm -rf ./release/")

local isRun = true
local osName = os.getenv("GOOS")
local versionName = "0.1"
assert(osName, "please set GOOS & GOARCH evn variable")

local function clearAll()
    files.delete('./target.html')
    files.delete('./target.go')
end
clearAll()

local hBuilder = HtmlBuilder(false)
hBuilder:inputFile("./terminal.html")
hBuilder:containScript()
hBuilder:containStyle()
hBuilder:containImage()
hBuilder:setOutput("./target.html")
hBuilder:start()

local cBuilder = CodeBuilder(false)
cBuilder:setInput("./terminal.go")
cBuilder:handleMacro("//")
cBuilder:setCallback(function(code, firsArg)
    if firsArg ~= "VERSION_NAME" then
        return string.format(code, "v" .. versionName)
    end
end)
cBuilder:setOutput("./target.go")
cBuilder:start()

if isRun then
    os.execute("go run target.go")
else
    files.mk_folder("./release/")
    os.execute("go build target.go")
    local fileExt = osName == "windows" and ".exe" or ""
    local targetName = "./target" .. fileExt
    local releaseName = "./release/simple_web_terminal_" .. versionName .. "_" .. osName .. fileExt
    files.copy(targetName, releaseName)
    files.delete(targetName)
    clearAll()
end

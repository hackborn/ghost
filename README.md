# ghost

Ghost is a command-line app that can be used to provide auto-compile and hosting functionality, depending on configuration.

## to use

From a command prompt, navigate to the ghost directory and launch the app with at least one argument, the path to a configuration file. The config file will determine how the app works. There are currently three supplied default config files:

1. **host**

Host mode launches an application and keeps it running.

example (on windows): *ghost host -file=notepad.exe*

2. **gulp**

Gulp mode watches a folder. When a change is detected, it builds the application and then launches it in host mode. The supplied configuration file is used explicitly to gulp Go applications.

example: *ghost go_gulp -watch="C:\go\github.com\hackborn\ghost" -run="ghost.exe"*

3. **format gulp**

Format gulp works like gulp mode, but if formats the Go code before building it.

example: *ghost go_fmt_gulp -watch="C:\go\github.com\hackborn\ghost" -run="ghost.exe"*

## design

At heart it's a simple pipeline processor, where the pipeline is composed of any number of nodes run in series. There are currently two types of nodes: Watch, which watches a folder, and Exec, which runs a command. There's an additional node called Host, which is actually an Exec node configured to automatically run and rerun the Exec command.

## known issues

* The watcher will not add or remove folders from the watch list. You can add or remove files to watch folders, but if you add a new folder you want watched, you need to restart the app.
* Watcher has no way to disallow folders, it currently attempts to watch every folder under the root. The number of folders to watched is managed by having a filter, but if you don't include the filter, and have a large directory structure of files you don't care about, you end up with a large number of watched folders.
* The exec command will not run if it receives continuous messages from a watch. This needs to be improved a little so it always runs after a set time, even if there are more changes.
* App does not properly shut down when hitting the close button on a windows command prompt. Not sure if this impacts anything -- go seems to be cleaning up all the important parts.

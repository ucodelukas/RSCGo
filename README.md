# RSCGo

This project is being designed with security, simplicity, portability, and performance in mind.

The other RSClassic server implementations that I've seen that were written in Java are plagued by a number of issues, not the least 
of which being feature creep, performance problems due to reliance on the JVM, and memory leaks.

A large portion of the codebase they're based on was written over a decade ago by a team of hobbyists in an attempt to breathe life 
into a then-dying game, and only a few developers have bothered to try to really improve the code in between then and now, with 
varying degrees of success.

Bottom line, the codebases I've seen being used are sub-par and usually large, complex monstrosities.

RSCGo was designed to be simple, lightweight, and fast.  It was designed to leverage modern technologies to provide a simple yet
highly performant RSClassic server.  It should be able to handle large player loads with no issues, and it should be hardened against
securiy issues we've seen in practice on other RSClassic servers.  It should eventually behave just like Jagex's RSClassic
implementation had.

Currently, RSCGo supports logging in, supports multiple players, chat works, walking works, objects are where they should be, etc.

The only major missing features, to my knowledge, are: magic, and prayer

Here is a progress update picture:
![](./screenies/11-25-19.png?raw=true)

Please note, to run or install this software, you must have a working Go 1.11+ compiler installed, and a working C compiler, as this project uses CGo for its SQLite3 driver.
I use GCC 7.4.0 on my test machine, and Go 1.13.1, on AMD64 Linux 5.0.0-23 kernel.
## Usage
    go run cmd/server/main.go (-vv)

The -vv flags are optional and provide more verbose logging output.
I would only recommend using the run command to test it out, and would advise you to produce a binary and run that instead in production environments.

## Compilation
    go build -o bin/RSCGo(.exe) cmd/server/main.go
    
On Windows you will want to add the .exe, but on any UNIX-likes you would leave it out.
You can also make the binary significantly smaller by stripping debug symbols, e.g:
    go build -ldflags="-s -w" -o ./bin/RSCGo cmd/server/main.go

This software is distributed under the terms of the ISC license, which basically permits you to do as you please with this software, as long as, with any distribution, you provide my copyright/permission notice from the license, and give me proper credit.

Enjoy!


## Public RSCGo servers
    https://rscturmoil.com/

I have begun hosting a publicly available RSCGo instance, using the 204 HTML5 webclient written by misterhat, to provide an RSC experience just as it used to be, to the masses without needing to do any downloading, or entering personal information to start playing at the site above, rscturmoil.com

As it stands, I am uncertain what direction this public server is headed(PK oriented, replica oriented, etc); I am almost certain that there will end up being a variant put up that is a replica-like experience, along with a much improved sister site that implements helpful QoL updates and etc that Jagex had not, as the team sees fit...

If you like my software, you should come join us for a chat in discord: https://discord.gg/PqNQpqB      

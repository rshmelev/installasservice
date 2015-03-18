package installasservice

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"
)

type ServiceInstallerOptions struct {
	MaxShutdownTime time.Duration
	AppName         string
	CompanyName     string
	// ShutdownUrl string
}

// if "__installservice" param is found across options then app
// is entering "ServiceInstaller" mode and creates /etc/init.d script
// that will start it with all passed command line arguments under nohup
func ProbablyInstallAsService(opts *ServiceInstallerOptions) {
	args := os.Args
	isServiceInstaller := false
	pos := 0
	for i, v := range args {
		if strings.HasPrefix(v, "__installservice") {
			isServiceInstaller = true
			pos = i
		}
	}
	if !isServiceInstaller {
		return
	}
	args = append(args[:pos], args[pos+1:]...)

	if runtime.GOOS == "windows" {
		log.Fatalln("installation as service under windows is not yet supported :(")
	}

	log.SetFlags(0)
	log.Println("...will modify /etc/init.d")
	beServiceInstaller(args, opts)

	// do not run the app!
	os.Exit(0)
}

func beServiceInstaller(args []string, opts *ServiceInstallerOptions) {
	if opts.MaxShutdownTime == time.Duration(0) {
		opts.MaxShutdownTime = time.Second * 5
	}

	fullappname := args[0]

	var basepath string
	if lia := strings.LastIndexAny(fullappname, "/\\"); lia > -1 {
		basepath = fullappname[:lia]
		fullappname = fullappname[lia+1:]
	}
	if basepath == "" || basepath == "." {
		basepath, _ = os.Getwd()
	}

	log.Println(Bold("installing " + opts.AppName + " service"))

	rawappname := fullappname
	fullappname, _ = RegexReplace(fullappname, "(_(debug|release|windows|linux|darwin|arm|386|amd64))*(.exe)?", "")

	params := ""
	if len(args) > 1 {
		params = sliceToCmdStr(args[1:])[1:]
	}

	if opts.AppName == "" {
		parts := strings.Split(fullappname, "-")
		if len(parts) > 1 {
			opts.CompanyName = parts[0]
			opts.AppName = parts[1]
		} else {
			opts.CompanyName = ""
			opts.AppName = fullappname
		}
	}

	initd := strings.Replace(initd_template, "~~", "`", -1)
	initd = strings.Replace(initd, "<Company>", opts.CompanyName, -1)
	initd = strings.Replace(initd, "<App>", opts.AppName, -1)
	initd = strings.Replace(initd, "<Executable>", rawappname, -1)
	initd = strings.Replace(initd, "<Params>", params, -1)
	initd = strings.Replace(initd, "<Basepath>", basepath, -1)
	initd = strings.Replace(initd, "\r\n", "\n", -1)

	initdfilename := "/etc/init.d/" + opts.AppName
	if errwrite := ioutil.WriteFile(initdfilename, []byte(initd), 0755); errwrite != nil {
		log.Fatalln("writing [" + Bold(initdfilename) + "] failed")
	}

	log.Println("ok, testing `service " + opts.AppName + " status`: ")
	servicestart := []string{opts.AppName, "status"}
	cmd := exec.Command("service", servicestart...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if runerr := cmd.Run(); runerr != nil {
		log.Println("got error [" + runerr.Error() + "] while running: service " + strings.Join(servicestart, " "))
	} else {
		log.Println("...output looks ok, right? should be like 'not running'")
	}

	log.Println("creating some handy scripts...")
	app := opts.AppName
	write(basepath+"/sstart", "start service", "service "+app+" start")
	write(basepath+"/sstop", "stop service", "service "+app+" stop")
	write(basepath+"/taillog", "watch logs", "cd "+basepath+"/logs && tail -n 100 -f `ls -lrt | awk '{file=$9}END{print file}'`")
	write(basepath+"/tailconsole", "watch console output", "cd "+basepath+"/ && tail -n 100 -f console.log")
	write(basepath+"/try", "just run the app as is", "./"+rawappname+" "+params)

	if b, err := ioutil.ReadFile("~/.bashrc"); err == nil {
		if !strings.Contains(string(b), "alias "+app) {
			f, e := os.OpenFile("~/.bashrc", os.O_APPEND, 0666)
			if e != nil {
				log.Println("adding '" + app + "' alias to ~/.bashrc")
				f.WriteString("alias " + app + "='cd " + basepath + "'\n")
				f.Close()
			}
		}
	}

	log.Println("you may run `" + ("service " + opts.AppName + " autostart") + "` to enable autostart of service")
}

func write(filename, help, data string) {
	databytes := []byte(data)
	if b, err := ioutil.ReadFile(filename); err != nil {
		if b == nil || bytes.Compare(b, databytes) != 0 {
			log.Println(" - " + Bold(path.Base(filename)) + " - " + help)
			ioutil.WriteFile(filename, []byte(data), 744)
		}
	}
}

var initd_template string = `#!/bin/bash
#
# Service script for <Company>-<App>
#
# chkconfig: - 80 20
# description: <Company>-<App>
#
#### BEGIN INIT INFO
# Provides:          <App>
# Required-Start:    $syslog $time $network $local_fs $remote_fs
# Required-Stop:     $syslog $time $network $local_fs $remote_fs
# Default-Start:     3 4 5
# Default-Stop:      S 0 1 2 6
# Short-Description: <Company>-<App> service
# Description:       <Company>-<App> service
### END INIT INFO

# template author: rshmelev@gmail.com
# usage of this template: 
#   replaceall <App>, <Company>, <Executable>, <Basepath>, <Params> 
#   copy to /etc/rc.d/init.d (or smth like that on your OS), 
#   chmod +x <App>
#   .. to make it run on system startup:
#   sudo service <App> autostart

COMPANY=<Company>
APPNAME=<App>
EXECUTABLE=<Executable>
EXECPARAMS=<Params>
# without trailing slash
BASEPATH="<Basepath>"

MAX_STOP_TIME_SEC=5
MAX_START_TIME_SEC=10

#-----------------------------------------------

if [[ -z $COMPANY ]] ; then
    FULLNAME=$COMPANY-$APPNAME
else
    FULLNAME=$APPNAME
fi
if [[ -z $EXECUTABLE ]] ; then
    EXECUTABLE=$FULLNAME
fi
ACTION=${1}
RETVAL=0
#PIDFILE should be created by our app on startup...
PIDFILE="/var/run/$FULLNAME.pid"

#-------------------------------------------------

FUNCTIONS_EXIST=false
if [ -f /etc/rc.d/init.d/functions ] ; then
    . /etc/rc.d/init.d/functions
    FUNCTIONS_EXIST=true
fi
if [ -f /etc/init.d/functions ] ; then
    . /etc/init.d/functions
    FUNCTIONS_EXIST=true
fi

if ! $FUNCTIONS_EXIST ; then
    #create own replacements
    failure() { 
        echo "  [FAIL]"
    }
    success() { 
        echo "  [OK]"
    }
fi

start()
{
    if [ -f $PIDFILE ] ;
    then
        echo "$PIDFILE exists, cannot start"
        localstatus
        return 0
    fi

    rm -f $PIDFILE

    echo -n -e $"starting $FULLNAME .."
    echo "starting $FULLNAME $EXECPARAMS now.." >> /var/log/service-$APPNAME.log
    cd $BASEPATH
    nohup ./$EXECUTABLE $EXECPARAMS 2>&1 >> $BASEPATH/console.log &
    echo $! > $PIDFILE
    
    # actually this part is not needed for &
    # because we get PID immediately. 
    # some other cases will require service to create his pid/lock itself
    secondsCounter=0
    while [ ! -e "$PIDFILE" ] && [ $secondsCounter -le $MAX_START_TIME_SEC ]
    do
        sleep 1
        echo -n ".."
        let secondsCounter=$secondsCounter+1;
    done

    if [ $secondsCounter -gt $MAX_STOP_TIME_SEC ]; then
        echo -n -e "\nprocess didn't start in $MAX_START_TIME_SEC seconds\n"
        RETVAL=-1
    else
        success
    fi

    echo
    verbose_localstatus
    return 0
}

stop()
{
    echo "stopping $FULLNAME now.." >> /var/log/service-$APPNAME.log
    if [ -a $PIDFILE ]; then
        kpid=~~cat $PIDFILE~~
        if [ ~~ps -p $kpid | grep -c $kpid~~ = '0' ]; then
            echo -n "process $kpid does not exist, nothing to kill"
        else 
            echo -n "killing process $kpid.."
            kill -- -$(ps -o pgid= $kpid | tr -d ' ')
            if [ ~~ps -p $kpid | grep -c $kpid~~ = '0' ]; then
                success
            else
                failure
            fi
          
        fi
        
        secondsCounter=0;
        if [ -n "$kpid" ]; then
            until [ ~~ps -p $kpid | grep -c $kpid~~ = '0' ] || [ $secondsCounter -gt $MAX_STOP_TIME_SEC ]
            do
                echo -n -e "Waiting for process ( $kpid) to exit..\n";
                sleep 1
                let secondsCounter=$secondsCounter+1;
            done
        fi
        if [ $secondsCounter -gt $MAX_STOP_TIME_SEC ]; then
            echo -n -e "Killing process ($kpid) because it didn't stop after $MAX_STOP_TIME_SEC seconds\n"
            # SIGALRM
            kill -14 -- -$(ps -o pgid= $kpid | tr -d ' ')
            sleep 1
            # ehh...
            if [ ~~ps -p $kpid | grep -c $kpid~~ = '0' ]; then
                echo "looks like it finally shut down"
            else 
                echo "have to use SIGKILL..."
                kill -9 -- -$(ps -o pgid= $kpid | tr -d ' ')
            fi
        else
            rm -f $PIDFILE    
        fi
    else
        verbose_localstatus
    fi
 
    rm -f $PIDFILE
    echo 
    return 0
}

verbose_localstatus() {
    localstatus
    if [ "$status" = "crashed" ]; then
        echo "not running but pid file exists"
    fi
    if [ "$status" = "running" ]; then
        echo "running with pid $PID"
    fi
    if [ "$status" = "stopped" ]; then
        echo "not running currently"
    fi
}

localstatus() {
    if [ -f $PIDFILE ]; then
        PID=~~cat $PIDFILE~~
        if [ -z "~~ps -ef | grep $PID | grep -v grep~~" ];
        then
            status="crashed"
        else
            status="running"
        fi
    else
        status="stopped"
    fi
}

# router
case "$ACTION" in
start)
    start
    ;;
stop)
    echo -n -e $"stopping $FULLNAME..\n"
    stop
    ;;
status)
    verbose_localstatus
    ;;
restart)
    echo -n -e "restarting $FULLNAME..\n"
    stop
    start
    ;;
autostart)
    echo "service $FULLNAME will run on system startup now.."
    command -v chkconfig >/dev/null 2>&1 && chkconfig --add $APPNAME
    command -v update-rc.d >/dev/null 2>&1 && update-rc.d $APPNAME defaults >/dev/null
    ;;
*)
    echo -n -e $"Usage: $APPNAME {start|stop|restart|status|autostart}\n"
    exit 1
esac

exit $RETVAL
`

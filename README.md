Golang lib to make building Go service easy.    
Works under Linux only! Modifies /etc/init.d   
Should work for CentOS and Debian, however... not tested well for now :)     
Library provides function to install your app and make it support common functionality as:  
service <app> start/stop/restart/status
Library is not tested well for now so I suggest to not use it in production!

Also, right now there's no flag to make it not log all app output to non-rotating file.   
Will definitely implement it later :).    

Questions?  
rshmelev@gmail.com  


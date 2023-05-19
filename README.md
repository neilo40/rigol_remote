# VISA
Good god what a pile of crap
1. Get the repo details from http://www.ni.com/downloads/ni-drivers/
2. apt update; apt install ni-visa
3. reboot
4. hack /home/neil/git/rigol_remote/vendor/github.com/jpoirier/visa/visa.go to add #cgo CFLAGS: -I/usr/include/ni-visa
5. regenerate cgo definitions in that file

# links
https://www.batronix.com/files/Rigol/Oszilloskope/_DS&MSO1000Z/MSO_DS1000Z_ProgrammingGuide_EN.pdf

# Notes
 * Cannot have usb and LAN at the same time on the scope.  RemoteIO must have LAN=on
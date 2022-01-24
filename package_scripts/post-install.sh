#!/bin/sh

# Step 1, decide if we should use systemd or init/upstart
use_systemctl="True"
systemd_version=0
if ! command -V systemctl >/dev/null 2>&1; then
  use_systemctl="False"
else
    systemd_version=$(systemctl --version | head -1 | sed 's/systemd //g' | sed 's/(.*//')
fi

cleanup() {
    # This is where you remove files that were not needed on this platform / system
    if [ "${use_systemctl}" = "False" ]; then
        rm -f /lib/systemd/system/jaxnetd.service
    else
        rm -f /etc/chkconfig/jaxnetd
        rm -f /etc/init.d/jaxnetd
    fi
}

install() {
    printf "\033[32m Post Install of an clean install\033[0m\n"
    if [ "${use_systemctl}" = "False" ]; then
        if command -V chkconfig >/dev/null 2>&1; then
          chkconfig --add jaxnetd
        fi

        service jaxnetd restart ||:
    else
        # rhel/centos7 cannot use ExecStartPre=+ to specify the pre start should be run as root
        # even if you want your service to run as non root.
        if [ "${systemd_version}" -lt 231 ]; then
            printf "\033[31m systemd version %s is less then 231, fixing the service file \033[0m\n" "${systemd_version}"
            sed -i "s/=+/=/g" /lib/systemd/system/jaxnetd.service
        fi
        printf "\033[32m Reload the service unit from disk\033[0m\n"
        systemctl daemon-reload ||:
        printf "\033[32m Unmask the service\033[0m\n"
        systemctl unmask jaxnetd ||:
        printf "\033[32m Set the preset flag for the service unit\033[0m\n"
        systemctl preset jaxnetd ||:
        printf "\033[32m The service file has been created for you.\033[0m\n"
        printf "\033[32m The service, however, has not been started or enabled\033[0m\n"
        printf "\033[32mIf you need it just run \"systemctl enable jaxnetd\" and \"systemctl restart jaxnetd\"\033[0m\n"
    fi
}

upgrade() {
    printf "\033[32m Post Install of an upgrade\033[0m\n"
    # Step 3(upgrade), do what you need
    ...
}

install
# Step 4, clean up unused files, yes you get a warning when you remove the package, but that is ok.
cleanup
  
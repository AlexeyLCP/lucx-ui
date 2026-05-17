#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

xui_folder="${XUI_MAIN_FOLDER:=/usr/local/x-ui}"
xui_service="${XUI_SERVICE:=/etc/systemd/system}"

# check root
[[ $EUID -ne 0 ]] && echo -e "${red}Критическая ошибка: ${plain} Запустите скрипт с правами root \n " && exit 1

# Check OS and set release variable
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    echo "Не удалось определить ОС. Обратитесь к автору!" >&2
    exit 1
fi
echo "Версия ОС: $release"

arch() {
    case "$(uname -m)" in
        x86_64 | x64 | amd64) echo 'amd64' ;;
        i*86 | x86) echo '386' ;;
        armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
        armv7* | armv7 | arm) echo 'armv7' ;;
        armv6* | armv6) echo 'armv6' ;;
        armv5* | armv5) echo 'armv5' ;;
        s390x) echo 's390x' ;;
        *) echo -e "${green}Неподдерживаемая архитектура CPU! ${plain}" && rm -f install.sh && exit 1 ;;
    esac
}

echo "Архитектура: $(arch)"

# Simple helpers
is_ipv4() {
    [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] && return 0 || return 1
}
is_ipv6() {
    [[ "$1" =~ : ]] && return 0 || return 1
}
is_ip() {
    is_ipv4 "$1" || is_ipv6 "$1"
}
is_domain() {
    [[ "$1" =~ ^([A-Za-z0-9](-*[A-Za-z0-9])*\.)+(xn--[a-z0-9]{2,}|[A-Za-z]{2,})$ ]] && return 0 || return 1
}

# Порт helpers
is_port_in_use() {
    local port="$1"
    if command -v ss > /dev/null 2>&1; then
        ss -ltn 2> /dev/null | awk -v p=":${port}$" '$4 ~ p {exit 0} END {exit 1}'
        return
    fi
    if command -v netstat > /dev/null 2>&1; then
        netstat -lnt 2> /dev/null | awk -v p=":${port} " '$4 ~ p {exit 0} END {exit 1}'
        return
    fi
    if command -v lsof > /dev/null 2>&1; then
        lsof -nP -iTCP:${port} -sTCP:LISTEN > /dev/null 2>&1 && return 0
    fi
    return 1
}

# LUCX-HOOK: Prompt with 10-second timeout — auto-selects default on timeout
read_prompt() {
    local prompt="$1"
    local default="$2"
    local var_name="$3"
    local timeout=10
    read -t $timeout -rp "$prompt" $var_name
    if [[ $? -ne 0 ]]; then
        eval $var_name="$default"
        echo ""
        echo -e "${yellow}Таймаут (${timeout}с) — используется значение по умолчанию: ${default}${plain}"
    fi
}

install_base() {
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf -y update && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl
            ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum -y update && yum install -y cronie curl tar tzdata socat ca-certificates openssl
            else
                dnf -y update && dnf install -y -q cronie curl tar tzdata socat ca-certificates openssl
            fi
            ;;
        arch | manjaro | parch)
            pacman -Syu && pacman -Syu --noconfirm cronie curl tar tzdata socat ca-certificates openssl
            ;;
        opensuse-tumbleweed | opensuse-leap)
            zypper refresh && zypper -q install -y cron curl tar timezone socat ca-certificates openssl
            ;;
        alpine)
            apk update && apk add dcron curl tar tzdata socat ca-certificates openssl
            ;;
        *)
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl
            ;;
    esac

    # LUCX-HOOK: Установка AWG/Telemt system dependencies
    echo -e "${green}Установка зависимостей LucX (iproute2, iptables, AWG)...${plain}"
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get install -y -q iproute2 iptables 2>/dev/null || true
            # AWG: build from source (pumbaX/awg-multi-script method)
            echo -e "${green}Установка сборочных зависимостей AWG...${plain}"
            apt-get install -y -q build-essential git libmnl-dev pkg-config dkms 2>/dev/null || true
            echo -e "${green}Установка заголовков ядра...${plain}"
            apt-get install -y -q "linux-headers-$(uname -r)" 2>/dev/null || \
                apt-get install -y -q linux-headers-amd64 2>/dev/null || \
                apt-get install -y -q linux-headers-generic 2>/dev/null || true
            # Build and install kernel module
            echo -e "${green}Сборка модуля ядра amneziawg...${plain}"
            local tmp_mod=/tmp/amneziawg-linux-kernel-module
            rm -rf "$tmp_mod"
            git clone --depth 1 https://github.com/amnezia-vpn/amneziawg-linux-kernel-module.git "$tmp_mod" 2>/dev/null && {
                cd "$tmp_mod/src"
                make dkms-install 2>/dev/null || true
                local mod_ver=$(grep -oP 'version\s*"\K[^"]+' dkms.conf 2>/dev/null || echo "1.0.0")
                dkms add -m amneziawg -v "$mod_ver" 2>/dev/null || true
                dkms build -m amneziawg -v "$mod_ver" 2>/dev/null || true
                dkms install -m amneziawg -v "$mod_ver" 2>/dev/null || true
                cd "$cur_dir"
                rm -rf "$tmp_mod"
            }
            # Build and install userspace tools
            echo -e "${green}Сборка утилит amneziawg (awg, awg-quick)...${plain}"
            local tmp_tools=/tmp/amneziawg-tools
            rm -rf "$tmp_tools"
            git clone --depth 1 https://github.com/amnezia-vpn/amneziawg-tools.git "$tmp_tools" 2>/dev/null && {
                cd "$tmp_tools/src"
                make 2>/dev/null && make install 2>/dev/null || true
                cd "$cur_dir"
                rm -rf "$tmp_tools"
            }
            # Load module and enable autostart
            modprobe amneziawg 2>/dev/null || {
                echo -e "${yellow}┌──────────────────────────────────────────────────────┐${plain}"
                echo -e "${yellow}│ [ПРЕДУПРЕЖДЕНИЕ] Не удалось загрузить модуль ядра AWG. │${plain}"
                echo -e "${yellow}│ Панель работает — AWG-инбаунды настраиваются вручную.  │${plain}"
                echo -e "${yellow}└──────────────────────────────────────────────────────┘${plain}"
            }
            if lsmod | grep -qE '^amneziawg\s' 2>/dev/null; then
                echo "amneziawg" > /etc/modules-load.d/amneziawg.conf 2>/dev/null || true
                echo -e "${green}AWG установлен и загружен успешно${plain}"
            fi
            ;;
        centos | fedora | rhel | almalinux | rocky | ol)
            yum install -y -q iproute iptables kernel-headers git dkms make gcc libmnl-devel 2>/dev/null || true
            echo -e "${yellow}AWG: сборка под RPM не автоматизирована — установите вручную из${plain}"
            echo -e "${yellow}https://github.com/amnezia-vpn/amneziawg-linux-kernel-module${plain}"
            ;;
        arch | manjaro)
            pacman -S --noconfirm iproute2 iptables linux-headers git dkms make gcc libmnl 2>/dev/null || true
            echo -e "${yellow}AWG: сборка под Arch не автоматизирована — установите вручную из${plain}"
            echo -e "${yellow}https://github.com/amnezia-vpn/amneziawg-linux-kernel-module${plain}"
            ;;
    esac
    # END LUCX-HOOK
}

gen_random_string() {
    local length="$1"
    openssl rand -base64 $((length * 2)) \
        | tr -dc 'a-zA-Z0-9' \
        | head -c "$length"
}

install_acme() {
    echo -e "${green}Установка acme.sh для управления SSL-сертификатами...${plain}"
    cd ~ || return 1
    curl -s https://get.acme.sh | sh > /dev/null 2>&1
    if [ $? -ne 0 ]; then
        echo -e "${red}Не удалось установить acme.sh${plain}"
        return 1
    else
        echo -e "${green}acme.sh успешно установлен${plain}"
    fi
    return 0
}

setup_ssl_certificate() {
    local domain="$1"
    local server_ip="$2"
    local existing_port="$3"
    local existing_webBasePath="$4"

    echo -e "${green}Настройка SSL-сертификата...${plain}"

    # Check if acme.sh is installed
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        install_acme
        if [ $? -ne 0 ]; then
            echo -e "${yellow}Не удалось установить acme.sh, skipping SSL setup${plain}"
            return 1
        fi
    fi

    # Create certificate directory
    local certPath="/root/cert/${domain}"
    mkdir -p "$certPath"

    # Issue certificate
    echo -e "${green}Выпуск SSL-сертификата для ${domain}...${plain}"
    echo -e "${yellow}Примечание: порт 80 должен быть открыт и доступен из интернета${plain}"

    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force > /dev/null 2>&1
    ~/.acme.sh/acme.sh --issue -d ${domain} --listen-v6 --standalone --httpport 80 --force

    if [ $? -ne 0 ]; then
        echo -e "${yellow}Не удалось выпустить сертификат для ${domain}${plain}"
        echo -e "${yellow}Убедитесь, что порт 80 открыт, и попробуйте позже через: x-ui${plain}"
        rm -rf ~/.acme.sh/${domain} 2> /dev/null
        rm -rf "$certPath" 2> /dev/null
        return 1
    fi

    # Установка certificate
    ~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem \
        --reloadcmd "systemctl restart x-ui" > /dev/null 2>&1

    if [ $? -ne 0 ]; then
        echo -e "${yellow}Не удалось установить сертификат${plain}"
        return 1
    fi

    # Enable auto-renew
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade > /dev/null 2>&1
    # Secure permissions: private key readable only by owner
    chmod 600 $certPath/privkey.pem 2> /dev/null
    chmod 644 $certPath/fullchain.pem 2> /dev/null

    # Set certificate for panel
    local webCertFile="/root/cert/${domain}/fullchain.pem"
    local webKeyFile="/root/cert/${domain}/privkey.pem"

    if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
        ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile" > /dev/null 2>&1
        echo -e "${green}SSL-сертификат установлен и настроен успешно!${plain}"
        return 0
    else
        echo -e "${yellow}Файлы сертификата не найдены${plain}"
        return 1
    fi
}

# Issue Let's Encrypt IP certificate with shortlived profile (~6 days validity)
# Requires acme.sh and port 80 open for HTTP-01 challenge
setup_ip_certificate() {
    local ipv4="$1"
    local ipv6="$2" # optional

    echo -e "${green}Настройка IP-сертификата Let's Encrypt (краткосрочный профиль)...${plain}"
    echo -e "${yellow}Примечание: IP-сертификаты действительны ~6 дней и автообновляются.${plain}"
    echo -e "${yellow}Порт по умолчанию — 80. При выборе другого порта обеспечьте форвард внешнего порта 80 на него.${plain}"

    # Check for acme.sh
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        install_acme
        if [ $? -ne 0 ]; then
            echo -e "${red}Не удалось установить acme.sh${plain}"
            return 1
        fi
    fi

    # Validate IP address
    if [[ -z "$ipv4" ]]; then
        echo -e "${red}Требуется IPv4-адрес${plain}"
        return 1
    fi

    if ! is_ipv4 "$ipv4"; then
        echo -e "${red}Неверный IPv4-адрес: $ipv4${plain}"
        return 1
    fi

    # Create certificate directory
    local certDir="/root/cert/ip"
    mkdir -p "$certDir"

    # Build domain arguments
    local domain_args="-d ${ipv4}"
    if [[ -n "$ipv6" ]] && is_ipv6 "$ipv6"; then
        domain_args="${domain_args} -d ${ipv6}"
        echo -e "${green}Включая IPv6-адрес: ${ipv6}${plain}"
    fi

    # Set reload command for auto-renewal (add || true so it doesn't fail during first install)
    local reloadCmd="systemctl restart x-ui 2>/dev/null || rc-service x-ui restart 2>/dev/null || true"

    # Choose port for HTTP-01 listener (default 80, prompt override)
    local WebPort=""
    read -rp "Порт для ACME HTTP-01 (по умолчанию 80): " WebPort
    WebPort="${WebPort:-80}"
    if ! [[ "${WebPort}" =~ ^[0-9]+$ ]] || ((WebПорт < 1 || WebПорт > 65535)); then
        echo -e "${red}Неверный порт. Используется 80.${plain}"
        WebPort=80
    fi
    echo -e "${green}Используется порт ${WebPort} для standalone-проверки.${plain}"
    if [[ "${WebPort}" -ne 80 ]]; then
        echo -e "${yellow}Напоминание: Let's Encrypt подключается на порт 80; направьте внешний порт 80 на ${WebPort}.${plain}"
    fi

    # Ensure chosen port is available
    while true; do
        if is_port_in_use "${WebPort}"; then
            echo -e "${yellow}Порт ${WebPort} занят.${plain}"

            local alt_port=""
            read -rp "Введите другой порт для acme.sh (оставьте пустым для отмены): " alt_port
            alt_port="${alt_port// /}"
            if [[ -z "${alt_port}" ]]; then
                echo -e "${red}Порт ${WebPort} занят; невозможно продолжить.${plain}"
                return 1
            fi
            if ! [[ "${alt_port}" =~ ^[0-9]+$ ]] || ((alt_port < 1 || alt_port > 65535)); then
                echo -e "${red}Неверный порт.${plain}"
                return 1
            fi
            WebPort="${alt_port}"
            continue
        else
            echo -e "${green}Порт ${WebPort} is free and ready для standalone-проверки.${plain}"
            break
        fi
    done

    # Issue certificate with shortlived profile
    echo -e "${green}Выпуск IP-сертификата для ${ipv4}...${plain}"
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force > /dev/null 2>&1

    ~/.acme.sh/acme.sh --issue \
        ${domain_args} \
        --standalone \
        --server letsencrypt \
        --certificate-profile shortlived \
        --days 6 \
        --httpport ${WebPort} \
        --force

    if [ $? -ne 0 ]; then
        echo -e "${red}Не удалось выпустить IP-сертификат${plain}"
        echo -e "${yellow}Убедитесь, что порт ${WebPort} доступен (или направлен с внешнего порта 80)${plain}"
        # Cleanup acme.sh data for both IPv4 and IPv6 if specified
        rm -rf ~/.acme.sh/${ipv4} 2> /dev/null
        [[ -n "$ipv6" ]] && rm -rf ~/.acme.sh/${ipv6} 2> /dev/null
        rm -rf ${certDir} 2> /dev/null
        return 1
    fi

    echo -e "${green}Сертификат выпущен успешно, установка...${plain}"

    # Установка certificate
    # Примечание: acme.sh may report "Reload error" and exit non-zero if reloadcmd fails,
    # but the cert files are still installed. We check for files instead of exit code.
    ~/.acme.sh/acme.sh --installcert -d ${ipv4} \
        --key-file "${certDir}/privkey.pem" \
        --fullchain-file "${certDir}/fullchain.pem" \
        --reloadcmd "${reloadCmd}" 2>&1 || true

    # Verify certificate files exist (don't rely on exit code - reloadcmd failure causes non-zero)
    if [[ ! -f "${certDir}/fullchain.pem" || ! -f "${certDir}/privkey.pem" ]]; then
        echo -e "${red}Файлы сертификата не найдены after installation${plain}"
        # Cleanup acme.sh data for both IPv4 and IPv6 if specified
        rm -rf ~/.acme.sh/${ipv4} 2> /dev/null
        [[ -n "$ipv6" ]] && rm -rf ~/.acme.sh/${ipv6} 2> /dev/null
        rm -rf ${certDir} 2> /dev/null
        return 1
    fi

    echo -e "${green}Файлы сертификата успешно установлены${plain}"

    # Enable auto-upgrade for acme.sh (ensures cron job runs)
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade > /dev/null 2>&1

    # Secure permissions: private key readable only by owner
    chmod 600 ${certDir}/privkey.pem 2> /dev/null
    chmod 644 ${certDir}/fullchain.pem 2> /dev/null

    # Configure panel to use the certificate
    echo -e "${green}Настройка путей сертификатов для панели...${plain}"
    ${xui_folder}/x-ui cert -webCert "${certDir}/fullchain.pem" -webCertKey "${certDir}/privkey.pem"

    if [ $? -ne 0 ]; then
        echo -e "${yellow}Предупреждение: не удалось настроить пути сертификатов автоматически${plain}"
        echo -e "${yellow}Файлы сертификатов находятся:${plain}"
        echo -e "  Сертификат: ${certDir}/fullchain.pem"
        echo -e "  Ключ:   ${certDir}/privkey.pem"
    else
        echo -e "${green}Пути сертификатов настроены успешно${plain}"
    fi

    echo -e "${green}IP-сертификат установлен и настроен успешно!${plain}"
    echo -e "${green}Сертификат действителен ~6 дней, автообновление через acme.sh (cron).${plain}"
    echo -e "${yellow}acme.sh автоматически обновит и перезагрузит x-ui до истечения срока.${plain}"
    return 0
}

# Comprehensive manual SSL certificate issuance via acme.sh
ssl_cert_issue() {
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep 'webBasePath:' | awk -F': ' '{print $2}' | tr -d '[:space:]' | sed 's#^/##')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep 'port:' | awk -F': ' '{print $2}' | tr -d '[:space:]')

    # check for acme.sh first
    if ! command -v ~/.acme.sh/acme.sh &> /dev/null; then
        echo "acme.sh could not be found. Установкаing now..."
        cd ~ || return 1
        curl -s https://get.acme.sh | sh
        if [ $? -ne 0 ]; then
            echo -e "${red}Не удалось установить acme.sh${plain}"
            return 1
        else
            echo -e "${green}acme.sh успешно установлен${plain}"
        fi
    fi

    # get the domain here, and we need to verify it
    local domain=""
    while true; do
        read -rp "Введите доменное имя: " domain
        domain="${domain// /}" # Trim whitespace

        if [[ -z "$domain" ]]; then
            echo -e "${red}Домен не может быть пустым. Попробуйте снова.${plain}"
            continue
        fi

        if ! is_domain "$domain"; then
            echo -e "${red}Неверный формат домена: ${domain}. Введите корректное доменное имя.${plain}"
            continue
        fi

        break
    done
    echo -e "${green}Ваш домен: ${domain}, проверка...${plain}"
    SSL_ISSUED_DOMAIN="${domain}"

    # detect existing certificate and reuse it if present
    local cert_exists=0
    if ~/.acme.sh/acme.sh --list 2> /dev/null | awk '{print $1}' | grep -Fxq "${domain}"; then
        cert_exists=1
        local certInfo=$(~/.acme.sh/acme.sh --list 2> /dev/null | grep -F "${domain}")
        echo -e "${yellow}Найден существующий сертификат для ${domain}, будет использован повторно.${plain}"
        [[ -n "${certInfo}" ]] && echo "$certInfo"
    else
        echo -e "${green}Домен готов к выпуску сертификата...${plain}"
    fi

    # create a directory for the certificate
    certPath="/root/cert/${domain}"
    if [ ! -d "$certPath" ]; then
        mkdir -p "$certPath"
    else
        rm -rf "$certPath"
        mkdir -p "$certPath"
    fi

    # get the port number for the standalone server
    local WebPort=80
    read -rp "Выберите порт (по умолчанию 80): " WebPort
    if [[ ${WebPort} -gt 65535 || ${WebPort} -lt 1 ]]; then
        echo -e "${yellow}Введённый порт ${WebPort} некорректен, используется порт 80.${plain}"
        WebPort=80
    fi
    echo -e "${green}Будет использован порт: ${WebPort} to issue certificates. Убедитесь, что порт открыт.${plain}"

    # Остановка panel temporarily
    echo -e "${yellow}Временная остановка панели...${plain}"
    systemctl stop x-ui 2> /dev/null || rc-service x-ui stop 2> /dev/null

    if [[ ${cert_exists} -eq 0 ]]; then
        # issue the certificate
        ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force
        ~/.acme.sh/acme.sh --issue -d ${domain} --listen-v6 --standalone --httpport ${WebPort} --force
        if [ $? -ne 0 ]; then
            echo -e "${red}Выпуск сертификата не удался, проверьте логи.${plain}"
            rm -rf ~/.acme.sh/${domain}
            systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null
            return 1
        else
            echo -e "${green}Сертификат выпущен, установка...${plain}"
        fi
    else
        echo -e "${green}Используется существующий сертификат, установка...${plain}"
    fi

    # Setup reload command
    reloadCmd="systemctl restart x-ui || rc-service x-ui restart"
    echo -e "${green}Команда перезагрузки ACME по умолчанию: ${yellow}systemctl restart x-ui || rc-service x-ui restart${plain}"
    echo -e "${green}Эта команда будет выполняться при каждом выпуске и обновлении сертификата.${plain}"
    read -rp "Изменить команду перезагрузки ACME? (y/n): " setReloadcmd
    if [[ "$setReloadcmd" == "y" || "$setReloadcmd" == "Y" ]]; then
        echo -e "\n${green}\t1.${plain} Пресет: systemctl reload nginx ; systemctl restart x-ui"
        echo -e "${green}\t2.${plain} Ввести свою команду"
        echo -e "${green}\t0.${plain} Оставить по умолчанию"
        read -rp "Выберите вариант: " choice
        case "$choice" in
            1)
                echo -e "${green}Команда перезагрузки: systemctl reload nginx ; systemctl restart x-ui${plain}"
                reloadCmd="systemctl reload nginx ; systemctl restart x-ui"
                ;;
            2)
                echo -e "${yellow}Рекомендуется поместить x-ui restart в конец${plain}"
                read -rp "Введите свою команду перезагрузки: " reloadCmd
                echo -e "${green}Команда перезагрузки: ${reloadCmd}${plain}"
                ;;
            *)
                echo -e "${green}Оставлена команда по умолчанию${plain}"
                ;;
        esac
    fi

    # install the certificate
    local installOutput=""
    installOutput=$(~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem --reloadcmd "${reloadCmd}" 2>&1)
    local installRc=$?
    echo "${installOutput}"

    local installWroteFiles=0
    if echo "${installOutput}" | grep -q "Установкаing key to:" && echo "${installOutput}" | grep -q "Установкаing full chain to:"; then
        installWroteFiles=1
    fi

    if [[ -f "/root/cert/${domain}/privkey.pem" && -f "/root/cert/${domain}/fullchain.pem" && (${installRc} -eq 0 || ${installWroteFiles} -eq 1) ]]; then
        echo -e "${green}Сертификат установлен, включение автообновления...${plain}"
    else
        echo -e "${red}Ошибка установки сертификата, выход.${plain}"
        if [[ ${cert_exists} -eq 0 ]]; then
            rm -rf ~/.acme.sh/${domain}
        fi
        systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null
        return 1
    fi

    # enable auto-renew
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    if [ $? -ne 0 ]; then
        echo -e "${yellow}Проблемы с автообновлением, данные сертификата:${plain}"
        ls -lah /root/cert/${domain}/
        # Secure permissions: private key readable only by owner
        chmod 600 $certPath/privkey.pem 2> /dev/null
        chmod 644 $certPath/fullchain.pem 2> /dev/null
    else
        echo -e "${green}Автообновление настроено, данные сертификата:${plain}"
        ls -lah /root/cert/${domain}/
        # Secure permissions: private key readable only by owner
        chmod 600 $certPath/privkey.pem 2> /dev/null
        chmod 644 $certPath/fullchain.pem 2> /dev/null
    fi

    # start panel
    systemctl start x-ui 2> /dev/null || rc-service x-ui start 2> /dev/null

    # Prompt user to set panel paths after successful certificate installation
    read -rp "Установить этот сертификат для панели? (y/n): " setPanel
    if [[ "$setPanel" == "y" || "$setPanel" == "Y" ]]; then
        local webCertFile="/root/cert/${domain}/fullchain.pem"
        local webKeyFile="/root/cert/${domain}/privkey.pem"

        if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
            ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
            echo -e "${green}Пути сертификатов заданы для панели${plain}"
            echo -e "${green}Файл сертификата: $webCertFile${plain}"
            echo -e "${green}Файл приватного ключа: $webKeyFile${plain}"
            echo ""
            echo -e "${green}URL доступа: https://${domain}:${existing_port}/${existing_webBasePath}${plain}"
            echo -e "${yellow}Панель будет перезапущена для применения SSL...${plain}"
            systemctl restart x-ui 2> /dev/null || rc-service x-ui restart 2> /dev/null
        else
            echo -e "${red}Ошибка: сертификат или ключ не найден для домена: $domain.${plain}"
        fi
    else
        echo -e "${yellow}Пропуск настройки путей панели.${plain}"
    fi

    return 0
}

# Reusable interactive SSL setup (domain or IP)
# Sets global `SSL_HOST` to the chosen domain/IP for Access URL usage
prompt_and_setup_ssl() {
    local panel_port="$1"
    local web_base_path="$2"
    local server_ip="$3"

    local ssl_choice=""
    SSL_SCHEME="https"

    echo -e "${yellow}Выберите способ настройки SSL-сертификата:${plain}"
    echo -e "${green}1.${plain} Let's Encrypt для домена (90 дней, автообновление)"
    echo -e "${green}2.${plain} Let's Encrypt для IP-адреса (6 дней, автообновление)"
    echo -e "${green}3.${plain} Свой SSL-сертификат (путь к файлам)"
    echo -e "${green}4.${plain} Пропустить SSL (только за reverse proxy / SSH-туннелем)"
    echo -e "${blue}Примечание:${plain} Варианты 1 и 2 требуют открытый порт 80. Вариант 3 требует ручного указания путей."
    echo -e "${blue}Примечание:${plain} Вариант 4: панель по HTTP — безопасно только за nginx/Caddy или SSH-туннелем."
    read_prompt "Вариант SSL (10с — по умолчанию: 2=IP, 1=Домен, 3=Свой, 4=Пропустить): " "2" ssl_choice
    ssl_choice="${ssl_choice// /}" # Trim whitespace

    # Default to 2 (IP cert) if input is empty or invalid (not 1, 3 or 4)
    if [[ "$ssl_choice" != "1" && "$ssl_choice" != "3" && "$ssl_choice" != "4" ]]; then
        ssl_choice="2"
    fi

    case "$ssl_choice" in
        1)
            # User chose Let's Encrypt domain option
            echo -e "${green}Используется Let's Encrypt для домена...${plain}"
            if ssl_cert_issue; then
                local cert_domain="${SSL_ISSUED_DOMAIN}"
                if [[ -z "${cert_domain}" ]]; then
                    cert_domain=$(~/.acme.sh/acme.sh --list 2> /dev/null | tail -1 | awk '{print $1}')
                fi

                if [[ -n "${cert_domain}" ]]; then
                    SSL_HOST="${cert_domain}"
                    echo -e "${green}✓ SSL-сертификат настроен с доменом: ${cert_domain}${plain}"
                else
                    echo -e "${yellow}Настройка SSL, возможно, завершена, но домен не извлечён${plain}"
                    SSL_HOST="${server_ip}"
                fi
            else
                echo -e "${red}Ошибка настройки SSL для домена.${plain}"
                SSL_HOST="${server_ip}"
            fi
            ;;
        2)
            # User chose Let's Encrypt IP certificate option
            echo -e "${green}Используется Let's Encrypt для IP (краткосрочный профиль)...${plain}"

            # Ask for optional IPv6
            local ipv6_addr=""
            read -rp "Есть IPv6-адрес? (оставьте пустым для пропуска): " ipv6_addr
            ipv6_addr="${ipv6_addr// /}" # Trim whitespace

            # Остановка panel if running (port 80 needed)
            if [[ $release == "alpine" ]]; then
                rc-service x-ui stop > /dev/null 2>&1
            else
                systemctl stop x-ui > /dev/null 2>&1
            fi

            setup_ip_certificate "${server_ip}" "${ipv6_addr}"
            if [ $? -eq 0 ]; then
                SSL_HOST="${server_ip}"
                echo -e "${green}✓ IP-сертификат Let's Encrypt настроен успешно${plain}"
            else
                echo -e "${red}✗ Ошибка настройки IP-сертификата. Проверьте порт 80.${plain}"
                SSL_HOST="${server_ip}"
            fi
            ;;
        3)
            # User chose Custom Paths (User Provided) option
            echo -e "${green}Используется существующий сертификат...${plain}"
            local custom_cert=""
            local custom_key=""
            local custom_domain=""

            # 3.1 Request Domain to compose Panel URL later
            read -rp "Введите домен, для которого выпущен сертификат: " custom_domain
            custom_domain="${custom_domain// /}" # Remove spaces

            # 3.2 Loop for Certificate Path
            while true; do
                read -rp "Введите путь к сертификату (.crt / fullchain): " custom_cert
                # Strip quotes if present
                custom_cert=$(echo "$custom_cert" | tr -d '"' | tr -d "'")

                if [[ -f "$custom_cert" && -r "$custom_cert" && -s "$custom_cert" ]]; then
                    break
                elif [[ ! -f "$custom_cert" ]]; then
                    echo -e "${red}Ошибка: файл не существует! Попробуйте снова.${plain}"
                elif [[ ! -r "$custom_cert" ]]; then
                    echo -e "${red}Ошибка: файл существует, но не читается (проверьте права)!${plain}"
                else
                    echo -e "${red}Ошибка: файл пуст!${plain}"
                fi
            done

            # 3.3 Loop for Private Key Path
            while true; do
                read -rp "Введите путь к приватному ключу (.key / privatekey): " custom_key
                # Strip quotes if present
                custom_key=$(echo "$custom_key" | tr -d '"' | tr -d "'")

                if [[ -f "$custom_key" && -r "$custom_key" && -s "$custom_key" ]]; then
                    break
                elif [[ ! -f "$custom_key" ]]; then
                    echo -e "${red}Ошибка: файл не существует! Попробуйте снова.${plain}"
                elif [[ ! -r "$custom_key" ]]; then
                    echo -e "${red}Ошибка: файл существует, но не читается (проверьте права)!${plain}"
                else
                    echo -e "${red}Ошибка: файл пуст!${plain}"
                fi
            done

            # 3.4 Apply Settings via x-ui binary
            ${xui_folder}/x-ui cert -webCert "$custom_cert" -webCertKey "$custom_key" > /dev/null 2>&1

            # Set SSL_HOST for composing Panel URL
            if [[ -n "$custom_domain" ]]; then
                SSL_HOST="$custom_domain"
            else
                SSL_HOST="${server_ip}"
            fi

            echo -e "${green}✓ Пути своего сертификата применены.${plain}"
            echo -e "${yellow}Примечание: You are responsible for renewing these files externally.${plain}"

            systemctl restart x-ui > /dev/null 2>&1 || rc-service x-ui restart > /dev/null 2>&1
            ;;
        4)
            echo ""
            echo -e "${red}⚠ Панель будет установлена БЕЗ SSL/TLS.${plain}"
            echo -e "${yellow}Учётные данные и cookie будут передаваться открытым HTTP.${plain}"
            echo -e "${yellow}Безопасно только когда:${plain}"
            echo -e "${yellow}  • Reverse proxy (nginx, Caddy, Traefik) терминирует TLS, или${plain}"
            echo -e "${yellow}  • Вы заходите в панель только через SSH-туннель${plain}"
            echo ""

            SSL_SCHEME="http"
            SSL_HOST="${server_ip}"

            local bind_local=""
            read -rp "Привязать панель только к 127.0.0.1? (рекомендуется — только SSH-туннель / reverse-proxy) [y/N]: " bind_local
            if [[ "$bind_local" == "y" || "$bind_local" == "Y" ]]; then
                ${xui_folder}/x-ui setting -listenIP "127.0.0.1" > /dev/null 2>&1
                SSL_HOST="127.0.0.1"
                echo -e "${green}✓ Панель привязана к 127.0.0.1. Недоступна из интернета.${plain}"
                echo ""
                echo -e "${green}SSH Порт Forwarding — open the panel from your local machine via:${plain}"
                echo -e "  Обычная SSH-команда:"
                echo -e "  ${yellow}ssh -L 2222:127.0.0.1:${panel_port} root@${server_ip}${plain}"
                echo -e "  Если используется SSH-ключ:"
                echo -e "  ${yellow}ssh -i <sshkeypath> -L 2222:127.0.0.1:${panel_port} root@${server_ip}${plain}"
                echo -e "  Затем откройте в браузере:"
                echo -e "  ${yellow}http://localhost:2222/${web_base_path}${plain}"
                echo ""
                echo -e "${yellow}Альтернатива: направьте reverse proxy (nginx/Caddy) на 127.0.0.1:${panel_port} и позвольте ему терминировать TLS.${plain}"
            else
                echo -e "${yellow}Панель будет слушать на всех интерфейсах по HTTP. Убедитесь, что перед ней стоит терминатор TLS.${plain}"
            fi

            systemctl restart x-ui > /dev/null 2>&1 || rc-service x-ui restart > /dev/null 2>&1
            echo -e "${green}✓ Настройка SSL пропущена.${plain}"
            ;;
        *)
            echo -e "${red}Неверный вариант. Пропуск настройки SSL.${plain}"
            SSL_HOST="${server_ip}"
            ;;
    esac
}

config_after_install() {
    local existing_hasDefaultCredential=$(${xui_folder}/x-ui setting -show true | grep -Eo 'hasDefaultCredential: .+' | awk '{print $2}')
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}' | sed 's#^/##')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    # Properly detect empty cert by checking if cert: line exists and has content after it
    local existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
    local URL_lists=(
        "https://api4.ipify.org"
        "https://ipv4.icanhazip.com"
        "https://v4.api.ipinfo.io/ip"
        "https://ipv4.myexternalip.com/raw"
        "https://4.ident.me"
        "https://check-host.net/ip"
    )
    local server_ip=""
    for ip_address in "${URL_lists[@]}"; do
        local response=$(curl -s -w "\n%{http_code}" --max-time 3 "${ip_address}" 2> /dev/null)
        local http_code=$(echo "$response" | tail -n1)
        local ip_result=$(echo "$response" | head -n-1 | tr -d '[:space:]"')
        if [[ "${http_code}" == "200" && "${ip_result}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
            server_ip="${ip_result}"
            break
        fi
    done

    if [[ -z "$server_ip" ]]; then
        echo -e "${yellow}Не удалось определить IP-адрес сервера автоматически.${plain}"
        while [[ -z "$server_ip" ]]; do
            read -rp "Введите публичный IPv4-адрес сервера: " server_ip
            server_ip="${server_ip// /}"
            if [[ ! "$server_ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                echo -e "${red}Неверный IPv4-адрес. Попробуйте снова.${plain}"
                server_ip=""
            fi
        done
    fi

    if [[ ${#existing_webBasePath} -lt 4 ]]; then
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_webBasePath=$(gen_random_string 18)
            local config_username=$(gen_random_string 10)
            local config_password=$(gen_random_string 10)

            read_prompt "Настроить порт панели? (10с — по умолчанию: случайный) [y/n]: " "n" config_confirm
            if [[ "${config_confirm}" == "y" || "${config_confirm}" == "Y" ]]; then
                read_prompt "Порт панели (10с — по умолчанию: случайный): " "$(shuf -i 1024-62000 -n 1)" config_port
                echo -e "${yellow}Your Panel Порт is: ${config_port}${plain}"
            else
                local config_port=$(shuf -i 1024-62000 -n 1)
                echo -e "${yellow}Сгенерирован случайный порт: ${config_port}${plain}"
            fi

            ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}"

            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     Настройка SSL-сертификата (РЕКОМЕНДУЕТСЯ)   ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}SSL настоятельно рекомендуется. Пропускайте только если reverse proxy${plain}"
            echo -e "${yellow}или SSH-туннель обрабатывает TLS за вас.${plain}"
            echo -e "${yellow}Let's Encrypt теперь поддерживает и домены, и IP-адреса!${plain}"
            echo ""

            prompt_and_setup_ssl "${config_port}" "${config_webBasePath}" "${server_ip}"

            # Retrieve the API token for display
            local config_apiToken=$(${xui_folder}/x-ui setting -getApiToken true | grep -Eo 'apiToken: .+' | awk '{print $2}')

            # Display final credentials and access information
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     Установка панели завершена!         ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "Логин:       ${config_username}"
            echo -e "Пароль:      ${config_password}"
            echo -e "${green}Порт:        ${config_port}${plain}"
            echo -e "${green}WebBasePath: ${config_webBasePath}${plain}"
            echo -e "URL доступа:  ${SSL_SCHEME}://${SSL_HOST}:${config_port}/${config_webBasePath}"
            echo -e "API Token:   ${config_apiToken}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}⚠ ВАЖНО: Сохраните эти данные в надёжном месте!${plain}"
            if [[ "$SSL_SCHEME" == "https" ]]; then
                echo -e "${yellow}⚠ SSL-сертификат: включён и настроен${plain}"
            else
                echo -e "${yellow}⚠ SSL-сертификат: пропущен — панель только по HTTP. Используйте reverse proxy или SSH-туннель.${plain}"
            fi
        else
            local config_webBasePath=$(gen_random_string 18)
            echo -e "${yellow}WebBasePath отсутствует или слишком короткий. Генерация нового...${plain}"
            ${xui_folder}/x-ui setting -webBasePath "${config_webBasePath}"
            echo -e "${green}Новый WebBasePath: ${config_webBasePath}${plain}"

            # If the panel is already installed but no certificate is configured, prompt for SSL now
            if [[ -z "${existing_cert}" ]]; then
                echo ""
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${green}     Настройка SSL-сертификата (РЕКОМЕНДУЕТСЯ)   ${plain}"
                echo -e "${green}═══════════════════════════════════════════${plain}"
                echo -e "${yellow}Let's Encrypt теперь поддерживает и домены, и IP-адреса!${plain}"
                echo ""
                prompt_and_setup_ssl "${existing_port}" "${config_webBasePath}" "${server_ip}"
                echo -e "${green}URL доступа:  ${SSL_SCHEME}://${SSL_HOST}:${existing_port}/${config_webBasePath}${plain}"
            else
                # If a cert already exists, just show the access URL
                echo -e "${green}URL доступа: https://${server_ip}:${existing_port}/${config_webBasePath}${plain}"
            fi
        fi
    else
        if [[ "$existing_hasDefaultCredential" == "true" ]]; then
            local config_username=$(gen_random_string 10)
            local config_password=$(gen_random_string 10)

            echo -e "${yellow}Обнаружены стандартные учётные данные. Обновление безопасности...${plain}"
            ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}"
            echo -e "Сгенерированы новые случайные учётные данные:"
            echo -e "###############################################"
            echo -e "Username: ${config_username}"
            echo -e "Password: ${config_password}"
            echo -e "###############################################"
        else
            echo -e "${green}Логин, пароль и WebBasePath настроены правильно.${plain}"
        fi

        # Existing install: if no cert configured, prompt user for SSL setup
        # Properly detect empty cert by checking if cert: line exists and has content after it
        existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
        if [[ -z "$existing_cert" ]]; then
            echo ""
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${green}     Настройка SSL-сертификата (РЕКОМЕНДУЕТСЯ)   ${plain}"
            echo -e "${green}═══════════════════════════════════════════${plain}"
            echo -e "${yellow}Let's Encrypt теперь поддерживает и домены, и IP-адреса!${plain}"
            echo ""
            prompt_and_setup_ssl "${existing_port}" "${existing_webBasePath}" "${server_ip}"
            echo -e "${green}URL доступа:  ${SSL_SCHEME}://${SSL_HOST}:${existing_port}/${existing_webBasePath}${plain}"
        else
            echo -e "${green}SSL-сертификат уже настроен. Действий не требуется.${plain}"
        fi
    fi

    ${xui_folder}/x-ui migrate
}

install_x-ui() {
    cd ${xui_folder%/x-ui}/

    # Download resources
    if [ $# == 0 ]; then
        # Use /releases (not /releases/latest) — /latest excludes pre-releases, returning 404
        tag_version=$(curl -Ls "https://api.github.com/repos/AlexeyLCP/lucx-ui/releases" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' | head -1)
        if [[ ! -n "$tag_version" ]]; then
            echo -e "${yellow}Попытка получить версию через IPv4...${plain}"
            tag_version=$(curl -4 -Ls "https://api.github.com/repos/AlexeyLCP/lucx-ui/releases" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' | head -1)
            if [[ ! -n "$tag_version" ]]; then
                echo -e "${red}Не удалось получить версию. Укажите версию явно:${plain}"
                echo -e "${yellow}  bash install-lucx.sh v0.1.5-pre-MVP${plain}"
                exit 1
            fi
        fi
        echo -e "Получена последняя версия LucX-UI: ${tag_version}, загрузка..."
        # Try API-based download first (bypasses GitHub CDN issues)
        local release_json=$(curl -s "https://api.github.com/repos/AlexeyLCP/lucx-ui/releases/tags/${tag_version}")
        local asset_url=$(echo "$release_json" | grep -o '"browser_download_url": *"[^"]*"' | grep -o 'https://[^"]*' | head -1)
        if [[ -n "$asset_url" ]]; then
            curl -4fLRo ${xui_folder}-linux-$(arch).tar.gz "$asset_url" 2>/dev/null || true
        fi
        # Fallback to direct URL
        if [[ ! -f ${xui_folder}-linux-$(arch).tar.gz ]]; then
            curl -4fLRo ${xui_folder}-linux-$(arch).tar.gz "https://github.com/AlexeyLCP/lucx-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz" 2>/dev/null || true
        fi
        if [[ ! -f ${xui_folder}-linux-$(arch).tar.gz ]]; then
            echo -e "${red}Ошибка загрузки. Файл релиза может быть недоступен.${plain}"
            echo -e "${yellow}Попробуйте собрать из исходников:${plain}"
            echo -e "${yellow}  git clone https://github.com/AlexeyLCP/lucx-ui${plain}"
            echo -e "${yellow}  cd lucx-ui/frontend && npm install && npm run build && cd ..${plain}"
            echo -e "${yellow}  go build -o x-ui . && sudo cp x-ui ${xui_folder}/${plain}"
            exit 1
        fi
    else
        tag_version=$1
        tag_version_numeric=${tag_version#v}
        min_version="0.1.0"

        if [[ "$(printf '%s\n' "$min_version" "$tag_version_numeric" | sort -V | head -n1)" != "$min_version" && "${tag_version_numeric}" != *"-"* ]]; then
            echo -e "${red}Используйте более новую версию (минимум v0.1.0). Выход.${plain}"
            exit 1
        fi

        url="https://github.com/AlexeyLCP/lucx-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz"
        echo -e "Начало установки x-ui $1"
        curl -4fLRo ${xui_folder}-linux-$(arch).tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Загрузка x-ui $1 не удалась, проверьте существование версии ${plain}"
            exit 1
        fi
    fi
    curl -4fLRo /usr/bin/x-ui-temp https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/x-ui.sh
    if [[ $? -ne 0 ]]; then
        echo -e "${red}Не удалось загрузить x-ui.sh${plain}"
        exit 1
    fi

    # Остановка x-ui service and remove old resources
    if [[ -e ${xui_folder}/ ]]; then
        if [[ $release == "alpine" ]]; then
            rc-service x-ui stop
        else
            systemctl stop x-ui
        fi
        rm ${xui_folder}/ -rf
    fi

    # Extract resources and set permissions
    tar zxvf x-ui-linux-$(arch).tar.gz
    rm x-ui-linux-$(arch).tar.gz -f

    cd x-ui
    chmod +x x-ui
    chmod +x x-ui.sh

    # Check the system's architecture and rename the file accordingly
    if [[ $(arch) == "armv5" || $(arch) == "armv6" || $(arch) == "armv7" ]]; then
        mv bin/xray-linux-$(arch) bin/xray-linux-arm
        chmod +x bin/xray-linux-arm
    fi
    chmod +x x-ui bin/xray-linux-$(arch)

    # Обновление x-ui cli and se set permission
    mv -f /usr/bin/x-ui-temp /usr/bin/x-ui
    chmod +x /usr/bin/x-ui
    mkdir -p /var/log/x-ui

    # LUCX-HOOK: Create LucX engine directories
    mkdir -p /etc/amnezia/amneziawg
    mkdir -p /etc/telemt /var/lib/telemt /var/run/telemt
    chmod 755 /etc/amnezia/amneziawg /etc/telemt /var/lib/telemt /var/run/telemt
    # END LUCX-HOOK
    config_after_install

    # Etckeeper compatibility
    if [ -d "/etc/.git" ]; then
        if [ -f "/etc/.gitignore" ]; then
            if ! grep -q "x-ui/x-ui.db" "/etc/.gitignore"; then
                echo "" >> "/etc/.gitignore"
                echo "x-ui/x-ui.db" >> "/etc/.gitignore"
                echo -e "${green}Добавлен x-ui.db в /etc/.gitignore для etckeeper${plain}"
            fi
        else
            echo "x-ui/x-ui.db" > "/etc/.gitignore"
            echo -e "${green}Создан /etc/.gitignore с x-ui.db для etckeeper${plain}"
        fi
    fi

    if [[ $release == "alpine" ]]; then
        curl -4fLRo /etc/init.d/x-ui https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/x-ui.rc
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Не удалось загрузить x-ui.rc${plain}"
            exit 1
        fi
        chmod +x /etc/init.d/x-ui
        rc-update add x-ui
        rc-service x-ui start
    else
        # Установка systemd service file
        service_installed=false

        if [ -f "x-ui.service" ]; then
            echo -e "${green}Найден x-ui.service в распакованных файлах, установка...${plain}"
            cp -f x-ui.service ${xui_service}/ > /dev/null 2>&1
            if [[ $? -eq 0 ]]; then
                service_installed=true
            fi
        fi

        if [ "$service_installed" = false ]; then
            case "${release}" in
                ubuntu | debian | armbian)
                    if [ -f "x-ui.service.debian" ]; then
                        echo -e "${green}Найден x-ui.service.debian, установка...${plain}"
                        cp -f x-ui.service.debian ${xui_service}/x-ui.service > /dev/null 2>&1
                        if [[ $? -eq 0 ]]; then
                            service_installed=true
                        fi
                    fi
                    ;;
                arch | manjaro | parch)
                    if [ -f "x-ui.service.arch" ]; then
                        echo -e "${green}Найден x-ui.service.arch, установка...${plain}"
                        cp -f x-ui.service.arch ${xui_service}/x-ui.service > /dev/null 2>&1
                        if [[ $? -eq 0 ]]; then
                            service_installed=true
                        fi
                    fi
                    ;;
                *)
                    if [ -f "x-ui.service.rhel" ]; then
                        echo -e "${green}Найден x-ui.service.rhel, установка...${plain}"
                        cp -f x-ui.service.rhel ${xui_service}/x-ui.service > /dev/null 2>&1
                        if [[ $? -eq 0 ]]; then
                            service_installed=true
                        fi
                    fi
                    ;;
            esac
        fi

        # If service file not found in tar.gz, download from GitHub
        if [ "$service_installed" = false ]; then
            echo -e "${yellow}Служебные файлы не найдены в архиве, загрузка с GitHub...${plain}"
            case "${release}" in
                ubuntu | debian | armbian)
                    curl -4fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/x-ui.service.debian > /dev/null 2>&1
                    ;;
                arch | manjaro | parch)
                    curl -4fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/x-ui.service.arch > /dev/null 2>&1
                    ;;
                *)
                    curl -4fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/x-ui.service.rhel > /dev/null 2>&1
                    ;;
            esac

            if [[ $? -ne 0 ]]; then
                echo -e "${red}Не удалось загрузить x-ui.service с GitHub${plain}"
                exit 1
            fi
            service_installed=true
        fi

        if [ "$service_installed" = true ]; then
            echo -e "${green}Настройка systemd-юнита...${plain}"
            chown root:root ${xui_service}/x-ui.service > /dev/null 2>&1
            chmod 644 ${xui_service}/x-ui.service > /dev/null 2>&1
            systemctl daemon-reload
            systemctl enable x-ui
            systemctl start x-ui
        else
            echo -e "${red}Не удалось установить файл x-ui.service${plain}"
            exit 1
        fi
    fi

    echo -e "${green}x-ui ${tag_version}${plain} установка завершена, панель запущена..."
    echo -e ""
    echo -e "┌───────────────────────────────────────────────────────┐
│  ${blue}Меню управления x-ui (подкоманды):${plain}              │
│                                                       │
│  ${blue}x-ui${plain}              - Скрипт управления          │
│  ${blue}x-ui start${plain}        - Запуск                            │
│  ${blue}x-ui stop${plain}         - Остановка                             │
│  ${blue}x-ui restart${plain}      - Перезапуск                          │
│  ${blue}x-ui status${plain}       - Текущий статус                   │
│  ${blue}x-ui settings${plain}     - Текущие настройки                 │
│  ${blue}x-ui enable${plain}       - Enable Autostart on OS Запускup   │
│  ${blue}x-ui disable${plain}      - Disable Autostart on OS Запускup  │
│  ${blue}x-ui log${plain}          - Просмотр логов                       │
│  ${blue}x-ui banlog${plain}       - Логи блокировок Fail2ban          │
│  ${blue}x-ui update${plain}       - Обновление                           │
│  ${blue}x-ui legacy${plain}       - Старая версия                   │
│  ${blue}x-ui install${plain}      - Установка                          │
│  ${blue}x-ui uninstall${plain}    - Удаление                        │
└───────────────────────────────────────────────────────┘"
}

echo -e "${green}Запуск...${plain}"
install_base
install_x-ui $1

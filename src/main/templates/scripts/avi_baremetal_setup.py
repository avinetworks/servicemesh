#!/usr/bin/python
#
# Script to Ease AVI Controller Installation
# This script assumes controller_docker.tgz(Controller image) in the /tmp directory
# Also assumes docker installed
# This script cleans and installs new images.
import logging, sys, os, re, socket, argparse, subprocess, traceback
sys.path.insert(0, '.')
RE_TAG = re.compile(r'Tag:.*', re.I)
log = None
se_link = ['/etc/rc0.d/K99avise_watcher', '/etc/rc1.d/K99avise_watcher',
           '/etc/rc2.d/S99avise_watcher', '/etc/rc3.d/S99avise_watcher',
           '/etc/rc4.d/S99avise_watcher', '/etc/rc5.d/S99avise_watcher',
           '/etc/rc6.d/K99avise_watcher']
con_link = ['/etc/rc0.d/K99avicontroller_watcher',
            '/etc/rc1.d/K99avicontroller_watcher', '/etc/rc2.d/S99avicontroller_watcher',
            '/etc/rc3.d/S99avicontroller_watcher', '/etc/rc4.d/S99avicontroller_watcher',
            '/etc/rc5.d/S99avicontroller_watcher', '/etc/rc6.d/K99avicontroller_watcher']

# Controller container image is around 3GB in size and the space it takes in /var/lib/docker (devicemapper data file) is around 4GB(image + rw layer + others).
# For SE the container image size is around 1 GB and the disk space it takes in /var/lib/docker is around 1.5GB
# In the latest images we have noticed another avi container apart from SE and Controller i.e. avinetworks/cli which takes around 500 MB in total
# On one host at a time we will have only 1 controller and 1 SE by default but when we upgrade the controller then we can end up with 2 controllers and 2 SE and then the next upgrade would take another controller space which would be around 3 GB. Hence totaling 3 controllers space (4GB *2 + 3GB) , 2 SEs space (1.5GB *2), 1 cli container (0.5GB), we landed up with the 15.5GB value. Adding 2.5 GB more as buffer. So total comes to 18 GB
# We dont require this total 18GB for a fresh install, but as of now validations of first install or different scenarios and making 18GB as the initial requirement is not done. In case some one hits a situation where /var/lib/docker is left with 10GB only and doesnt have any other way out they can change this value to a lower number in the script (but could face potential space issues when upgrade scenario comes for this controller)

# docker storage required size in KB (18 GB)
SIZE_REQ = 18*1024*1024
bash_var='/usr/bin/bash'

def execute_command(cmd):
    try:
        p = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, shell=True)
        out, err = p.communicate()
        log.debug('Executed the command %s\n output %s', cmd, out)
        if err:
            log.debug('Executed the command %s\n error occurred %s', cmd, err)
    except Exception as e:
        log.exception('Command %s execution failed %s traceback: %s', cmd, e, traceback.format_exc())
    return out.split('\n')[0]

def is_validip(address):
    if not address:
	return False
    try:
        socket.inet_aton(address)
        return True
    except socket.error:
        return False

def get_management_devname(mgmt_ip):
    # find devname of mgmt_ip
    dev_cmd  = ['ip', 'addr']
    grep_cmd = ['grep', mgmt_ip]
    dev_name = 'eth0'
    try:
        ip_p   = subprocess.Popen(dev_cmd, stdout=subprocess.PIPE)
        grep_p = subprocess.Popen(grep_cmd, stdin=ip_p.stdout, stdout=subprocess.PIPE)
        dev_line = grep_p.communicate()[0].strip()
        if dev_line:
            tmp = dev_line.split(' ')
            dev_name = tmp[-1].split(':')[0]
    except Exception as e:
        log.exception('%s Could not find Management interface device name %s', e, traceback.format_exc())
    return dev_name

def banner():
    log.info('\nWelcome to AVI Initialization Script\n\n'
            'Pre-requisites: This script assumes the below utilities are installed: \n'
            '                  docker (yum -y install docker/apt-get install docker.io) \n'
            'Supported Vers: OEL - 6.5,6.7,7.0,7.1,7.2,7.3,7.4 Centos/RHEL - 7.0,7.1,7.2,7.3,7.4, Ubuntu - 14.04,16.04\n')

def check_prereq():
    check_dist_ver()
    exist = int(execute_command('which docker | wc -l'))
    if not exist:
        log.error('Docker doesnt exist. Please Install docker')
        sys.exit(1)
    exist = int(execute_command('ps -ef | grep docker | grep -v grep | wc -l'))
    if not exist:
        log.error('Docker is not Running. Please Start Docker')
        sys.exit(1)

    validate_docker_storage()

def check_pkg(se, con):
    if se and not os.path.exists('se_docker.tgz'):
        log.error('Docker SE Image doesnt exist. Bad Package..')
        sys.exit(1)
    if con and not os.path.exists('/tmp/controller_docker.tgz'):
        log.error('Docker Controller Image doesnt exist. Bad Package..')
        sys.exit(1)

def check_dist_ver():
    dist = None
    dist_ver = None
    # dump avi version
    if os.path.exists('VERSION'):
        with open('VERSION') as f:
            m = RE_TAG.search(f.read())
            if m:
                v = m.group(0)
                log.info('AviVantage Version %s', v)
    if os.path.isfile('/etc/system-release'):
        try:
            with open('/etc/system-release') as f:
                s = f.read().lower()
                if (s.find('oracle linux') != -1) and (s.find('7.0') != -1):
                    dist = 'oel'
                    dist_ver = '7.0'
                elif (s.find('oracle linux') != -1) and (s.find('6.7') != -1):
                    dist = 'oel'
                    dist_ver = '6.7'
                elif (s.find('oracle linux') != -1) and (s.find('6.5') != -1):
                    dist = 'oel'
                    dist_ver = '6.5'
                elif (s.find('oracle linux') != -1) and (s.find('7.1') != -1):
                    dist = 'oel'
                    dist_ver = '7.1'
                elif (s.find('oracle linux') != -1) and (s.find('7.2') != -1):
                    dist = 'oel'
                    dist_ver = '7.2'
                elif (s.find('oracle linux') != -1) and (s.find('7.3') != -1):
                    dist = 'oel'
                    dist_ver = '7.3'
                elif (s.find('oracle linux') != -1) and (s.find('7.4') != -1):
                    dist = 'oel'
                    dist_ver = '7.4'
                elif (s.find('centos') != -1) and (s.find('7.0') != -1):
                    dist = 'centos'
                    dist_ver = '7.0'
                elif (s.find('centos') != -1) and (s.find('7.1') != -1):
                    dist = 'centos'
                    dist_ver = '7.1'
                elif (s.find('centos') != -1) and (s.find('7.2') != -1):
                    dist = 'centos'
                    dist_ver = '7.2'
                elif (s.find('centos') != -1) and (s.find('7.3') != -1):
                    dist = 'centos'
                    dist_ver = '7.3'
                elif (s.find('centos') != -1) and (s.find('7.4') != -1):
                    dist = 'centos'
                    dist_ver = '7.4'
                elif (s.find('red hat enterprise linux') != -1) and \
                     (s.find('7.0') != -1):
                    dist = 'rhel'
                    dist_ver = '7.0'
                elif (s.find('red hat enterprise linux') != -1) and \
                     (s.find('7.1') != -1):
                    dist = 'rhel'
                    dist_ver = '7.1'
                elif (s.find('red hat enterprise linux') != -1) and \
                     (s.find('7.2') != -1):
                    dist = 'rhel'
                    dist_ver = '7.2'
                elif (s.find('red hat enterprise linux') != -1) and \
                     (s.find('7.3') != -1):
                    dist = 'rhel'
                    dist_ver = '7.3'
                elif (s.find('red hat enterprise linux') != -1) and \
                     (s.find('7.4') != -1):
                    dist = 'rhel'
                    dist_ver = '7.4'
        except IOError:
            log.error('Linux Distribution unknown')
            sys.exit(1)
    else:
        # Check if this is ubuntu
        try:
            rel = execute_command('lsb_release -d')
            ver = execute_command('lsb_release -r')
            if 'Ubuntu' in rel:
                # This is ubuntu.
                global bash_var
                dist = 'ubuntu'
                dist_ver = ver.split()[1]
                bash_var = '/bin/bash'
        except:
            log.error('Linux Distribution not Ubuntu')
            sys.exit(1)
    if not dist or not dist_ver:
        log.error('Linux Distribution Unsupported')
        sys.exit(1)
    return dist, dist_ver

def cleanup_disks(service_file):
    run_params = execute_command("cat %s | sed -n -e 's/^.*docker run //p'" % service_file).split()
    for param in run_params:
        if 'opt/avi/' in param:
            host_path = param.split(':')[0]
            execute_command('rm -rf %s/*' % host_path)
            log.info('Cleaned up the disk %s', host_path)

def cleanup_images_se():
    se_systemd_service_file = '/etc/systemd/system/avise.service'
    se_initd_service_file = '/etc/init.d/avise'
    try:
        cexist = int(execute_command('docker ps -a | grep avinetworks/se: | grep -v CONTAINER | wc -l'))
        if cexist:
            log.info('Removing Existing AVI SE Docker Containers..')
            execute_command('docker stop `docker ps -a | grep avinetworks/se: | grep -v CONTAINER | awk \'{printf \"%s\\n\",$1}\' `')
            execute_command('docker rm -f `docker ps -a | grep avinetworks/se: | grep -v CONTAINER | awk \'{printf \"%s\\n\",$1}\' `')

        iexist = int(execute_command('docker images | grep ^avinetworks/se | wc -l'))
        if iexist:
            log.info('Removing Existing AVI SE Docker Images. Please Wait..')
            execute_command('docker rmi -f `docker images | grep ^avinetworks/se | grep -v REPOSITORY | awk \'{printf \"%s\\n\",$3}\' `')

	if os.path.exists(se_systemd_service_file):
            cleanup_disks(se_systemd_service_file)
            execute_command('rm -f %s' % se_systemd_service_file)
	elif os.path.exists(se_initd_service_file):
            cleanup_disks(se_initd_service_file)
            execute_command('rm -f %s' % se_initd_service_file)
            execute_command('rm -f /etc/init.d/avise_watcher')
            execute_command('rm -f /etc/init.d/avise_watcher.sh')

        log.info('Loading AVI SE Image. Please Wait..')
        execute_command('gunzip -c se_docker.tgz | docker load')
        se_image = execute_command('docker images | grep ^avinetworks/se | grep -v REPOSITORY | awk \'{printf \"%s:%s\",$1,$2}\' ')
        return se_image
    except:
        log.error('Unable to load Avi SE Image to Docker..')
        sys.exit(1)

def cleanup_images_con():
    con_systemd_service_file = '/etc/systemd/system/avicontroller.service'
    con_initd_service_file = '/etc/init.d/avicontroller'
    try:
        cexist = int(execute_command('docker ps -a | grep avinetworks/controller: | grep -v CONTAINER | wc -l'))
        if cexist:
            log.info('Removing Existing AVI CONTROLLER Docker Containers..')
            execute_command('docker stop `docker ps -a | grep avinetworks/controller: | grep -v CONTAINER | awk \'{printf \"%s\\n\",$1}\' `')
            execute_command('docker rm -f `docker ps -a | grep avinetworks/controller: | grep -v CONTAINER | awk \'{printf \"%s\\n\",$1}\' `')

        iexist = int(execute_command('docker images | grep ^avinetworks/controller | wc -l'))
        if iexist:
            log.info('Removing Existing AVI CONTROLLER Docker Images. Please Wait..')
            execute_command('docker rmi -f `docker images | grep ^avinetworks/controller | grep -v REPOSITORY | awk \'{printf \"%s\\n\",$3}\' `')

	if os.path.exists(con_systemd_service_file):
            cleanup_disks(con_systemd_service_file)
            execute_command('rm -f %s' % con_systemd_service_file)
	elif os.path.exists(con_initd_service_file):
            cleanup_disks(con_initd_service_file)
            execute_command('rm -f %s' % con_initd_service_file)
            execute_command('rm -f /etc/init.d/avicontroller_watcher')
            execute_command('rm -f /etc/init.d/avicontroller_watcher.sh')

        log.info('Loading AVI CONTROLLER Image. Please Wait..')
        execute_command('docker load -i /tmp/controller_docker.tgz')
        controller_image = execute_command('docker images | grep ^avinetworks/controller' + ' | grep -v REPOSITORY | awk \'{printf \"%s:%s\",$1,$2}\' ')
        return controller_image
    except:
        log.error('Unable to load Avi CONTROLLER Image to Docker..')
        sys.exit(1)

def _get_disk_avail(path):
    disk_avail = execute_command("df %s -BG | grep -iv Used | awk '{print $4}' | sed s/G//g" % path)
    if '%' in disk_avail:
        disk_avail = execute_command("df %s -BG | grep -iv Used | awk '{print $3}' | sed s/G//g" % path)
    disk_avail = int(disk_avail)
    return disk_avail

def _check_disk_cap(disk_size, disk_path):
    disk_avail = _get_disk_avail(disk_path)
    if disk_size > disk_avail:
        log.error('Not enough disk space available at %s, Use less than [%dG]', disk_path, disk_avail)
        sys.exit(1)

def _check_uniq_devices(disk_list, ctype='controller'):
    disk_set = set()
    for disk_size, disk_path in disk_list:
        if os.stat(disk_path).st_dev in disk_set:
            log.error('Specified multiple directory paths for %s. They need to be on unique partitions', ctype)
            sys.exit(1)
        disk_set.add(os.stat(disk_path).st_dev)

def _check_controller_disks(args):
    if not args['cdisk_path']:
        log.error('Need to specify the directory path for controller config data')
        sys.exit(1)
    if args['cdisk_path'] and args['cdiskm_path'] and not args['cdiskl_path']:
        log.error('Controller config and metrics directory paths provided. '
                'Need to specify the directory path for controller logs')
        sys.exit(1)
    disk_list = [disk_info for disk_info in [(args['cdisk'], args['cdisk_path']), (args['cdiskm'], args['cdiskm_path']), (args['cdiskl'], args['cdiskl_path'])]
                 if disk_info[1]]
    for disk_size, disk_path in disk_list:
        if not os.path.exists(disk_path):
            log.error('Directory path %s does not exist on this host', disk_path)
            sys.exit(1)
        _check_disk_cap(disk_size, disk_path)
    _check_uniq_devices(disk_list, 'controller')

def _check_se_disks(args):
    if not args['sdisk_path']:
        log.error('Need to specify the directory path for SE config data')
        sys.exit(1)
    disk_list = [disk_info for disk_info in [(args['sdisk'], args['sdisk_path']), (args['sdiskl'], args['sdiskl_path'])] if disk_info[1]]
    for disk_size, disk_path in disk_list:
        if not os.path.exists(disk_path):
            log.error('Directory path %s does not exist on this host', disk_path)
            sys.exit(1)
        _check_disk_cap(disk_size, disk_path)
    _check_uniq_devices(disk_list, 'se')

def get_se_containers_size():
    size_avail = 0
    se_cont_exist = int(execute_command('docker ps -a | grep avinetworks/se: | grep -v CONTAINER | wc -l'))
    if se_cont_exist:
        se_img_exist = execute_command("docker images | grep ^avinetworks/se | awk '{print $3}'")
        se_img_exist = se_img_exist.split()
        for image in se_img_exist:
            size_used = int(execute_command("docker inspect %s | grep -i '\"size\"' | awk '{print $2}' | sed s/,//g"%image))
            size_used /= 1024
            size_avail += size_used

            # Adding 512 MB to the avail, because docker will add extra r/w layer in /var/lib/docker
            size_avail += 512*1024
    return size_avail

def get_controller_containers_size():
    size_avail = 0
    con_cont_exist = int(execute_command('docker ps -a | grep avinetworks/controller: | grep -v CONTAINER | wc -l'))
    if con_cont_exist:
        con_img_exist = execute_command("docker images | grep ^avinetworks/controller | awk '{print $3}'")
        con_img_exist = con_img_exist.split()
        for image in con_img_exist:
            size_used = int(execute_command("docker inspect %s | grep -i '\"size\"' | awk '{print $2}' | sed s/,//g"%image))
            size_used /= 1024
            size_avail += size_used

            # Adding 1024 MB to the avail, because docker will add extra r/w layer in /var/lib/docker
            size_avail += 1024*1024
    return size_avail

def byte_converter(size_avail, data_format):
    if data_format.upper() == "TB":
        size_avail *= 1024*1024*1024
    elif data_format.upper() == "GB":
        size_avail *= 1024*1024
    elif data_format.upper() == "MB":
        size_avail *= 1024
    #For other size formats we can assume it as KB.
    return size_avail

def validate_docker_storage():
    size_avail = 0
    dock_disk_dir = execute_command("docker info | grep 'Data loop file:' | awk '{print $4}'")
    #For ubuntu the storage driver section is missing, need the below check
    if not dock_disk_dir:
        dock_disk_dir = execute_command("docker info | grep '^ Root Dir:' | awk '{print $3}'")
    #For "overlay" storage driver
    if not dock_disk_dir:
        dock_disk_dir = execute_command("docker info | grep 'Docker Root Dir:' | awk '{print $4}'")
    if dock_disk_dir:
        size_avail = execute_command("df %s | grep -iv Used | awk '{print $4}'" % dock_disk_dir)
        if size_avail:
            size_avail = int(size_avail)

    # If docker disk directory is not present
    if not dock_disk_dir:
        size_avail = execute_command("docker info | grep 'Data Space Available:' | awk '{print $4}'")
        if size_avail:
            size_avail = float(size_avail)
            data_format = execute_command("docker info | grep 'Data Space Available:' | awk '{print $5}'")
            if data_format:
                size_avail = byte_converter(size_avail, data_format)

    if not size_avail:
        size_avail = 0

    if size_avail >= SIZE_REQ:
        return
    else:
        size_avail += get_se_containers_size() + get_controller_containers_size()
        if size_avail >= SIZE_REQ:
            return
    add_format = 'MB'
    avail_format = 'MB'
    add_req = float((SIZE_REQ-size_avail)/(1024))
    if add_req > 1024:
        add_req = float(add_req/1024)
        add_format = 'GB'

    size_avail = float(size_avail/1024)
    if size_avail > 1024:
        size_avail = float(size_avail/1024)
        avail_format = 'GB'

    if dock_disk_dir:
        mount = execute_command("df %s | grep -iv Used | awk '{print $6}'" % dock_disk_dir)
        if mount:
            log.error('Disk storage is not enough to run in %s which is mounted on %s, available ~ %f %s, additional ~ %f %s more space required', dock_disk_dir, mount, size_avail, avail_format, add_req, add_format)
        else:
            log.error('Disk storage is not enough to run in %s, available ~ %f %s, additional ~ %f %s more space required', dock_disk_dir, size_avail, avail_format, add_req, add_format)
        sys.exit(1)
    elif size_avail:
        log.error('Disk storage is not enough to run, available ~ %f %s, additional ~ %f %s more space required', size_avail, avail_format, add_req, add_format)
        sys.exit(1)
    else:
        log.error("Docker disk path was not found, make sure docker data file eg:-/var/lib/docker/devicemapper/devicemapper/data partition has atleast 18GB free space")

def verify_cpu_memory(args):
    if args['start_con'] and args['start_se']:
        ncores = args['ccores'] + args['scores']
        nmem = (args['cmemg']*1024) + args['smem']
    elif args['start_con'] and not args['start_se']:
        ncores = args['ccores']
        nmem = (args['cmemg']*1024)
        se_cont_exist = int(execute_command('docker ps -a | grep avinetworks/se: | grep -v CONTAINER | wc -l'))
        if se_cont_exist:
            c_cpu = execute_command("docker exec avise env | grep NTHREADS | sed -e s/NTHREADS=//g | awk '{print $1}'")
            c_mem = execute_command("docker exec avise env | grep SEMEMMB | sed -e s/SEMEMMB=//g | awk '{print $1}'")
            c_cpu = int(c_cpu) if c_cpu else 0
            c_mem = int(c_mem) if c_mem else 0
            args['cmax'] -= c_cpu
            args['mmax'] -= c_mem
    elif not args['start_con'] and args['start_se']:
        ncores = args['scores']
        nmem = args['smem']
        con_cont_exist = int(execute_command('docker ps -a | grep avinetworks/controller: | grep -v CONTAINER | wc -l'))
        if con_cont_exist:
            c_cpu = execute_command("docker exec avicontroller env | grep NUM_CPU | sed -e s/NUM_CPU=//g | awk '{print $1}'")
            c_mem = execute_command("docker exec avicontroller env | grep NUM_MEMG | sed -e s/NUM_MEMG=//g | awk '{print $1}'")
            c_cpu = int(c_cpu) if c_cpu else 0
            c_mem = int(c_mem) if c_mem else 0
            c_mem *= 1024
            args['cmax'] -= c_cpu
            args['mmax'] -= c_mem
    else:
        log.error('Please Input to Run SE or Controller or Both')
        sys.exit(1)

    if args['cmax'] < ncores:
        log.error('The Cores are not enough to run. Controller Needs min 4 cores and SE min 1 core')
        sys.exit(1)
    if args['mmax'] < nmem:
        log.error('The Memory is not enough to run. Controller Needs min 12G and SE min 1G.\nRequested %d MB, '
                'Max Possible %d MB', nmem, args['mmax'])
        sys.exit(1)


def validate_input(args):
    check_existing_containers(args)
    verify_cpu_memory(args)

    if args['start_con'] and args['setup_json']:
        if not os.path.exists(args['setup_json']):
            log.error('Setup.json file %s doesnt exist', args['setup_json'])
            sys.exit(1)

    if args['start_con'] and args['start_se']:
        _check_controller_disks(args)
        _check_se_disks(args)
        if (args['ccores'] > (args['cmax'] - 1)) or (args['ccores'] < 4):
            log.error('Please Enter Valid Controller Cores Within Range [4, %d]', args['cmax'] - 1)
            sys.exit(1)
        if (args['cmemg'] > ((args['mmax']/1024) - 1)) or (args['cmemg'] < 4):
            log.error('Please Enter Controller Memory (in GB) Range [%d, %d]', 4, ((args['mmax']/1024) - 1))
            sys.exit(1)
        left_cores = args['cmax'] - args['ccores']
        left_mem = args['mmax'] - (args['cmemg']*1024)
        if (args['scores'] > left_cores) or (left_cores < 1):
            log.error('Please Enter Valid SE Cores Within Range [1, %d]', left_cores)
            sys.exit(1)
        smin = args['scores'] * 1024
        if smin > left_mem:
            log.error('Please Enter different number of SE cores as there isnt enough memory')
            sys.exit(1)
        if (args['smem'] > left_mem) or (args['smem'] < smin):
            log.error('Please Enter SE Memory (in MB) Range [%d, %d]', smin, left_mem)
            sys.exit(1)
        if not is_validip(args['cip']):
            log.error('Controller IP %s is not valid', args['cip'])
            sys.exit(1)
        if not is_validip(args['master_ctl']):
            log.error('Please Enter Valid IP')
            sys.exit(1)

    if args['start_con'] and (not args['start_se']):
        _check_controller_disks(args)
        if (args['ccores'] > args['cmax']) or (args['ccores'] < 4):
            log.error('Please Enter Controller Cores Within Range [4, %d]', args['cmax'])
            sys.exit(1)
        if (args['cmemg'] > (args['mmax']/1024)) or (args['cmemg'] < 4):
            log.error('Please Enter Controller Memory (in GB) Range [%d, %d]', 4, (args['mmax']/1024))
            sys.exit(1)
        if not is_validip(args['cip']):
            log.error('Controller IP %s is not valid', args['cip'])
            sys.exit(1)

    if (not args['start_con']) and args['start_se']:
        _check_se_disks(args)
        if (args['scores'] > args['cmax']) or (args['cmax'] < 1):
            log.error('Please Enter SE Cores Within Range [1, %d]', args['cmax'])
            sys.exit(1)
        smin = args['scores'] * 1024
        if smin > args['mmax']:
            log.error('Please Enter different number of SE cores as there isnt enough memory')
            sys.exit(1)
        if (args['smem'] > args['mmax']) or (args['smem'] < smin):
            log.error('Please Enter SE Memory (in MB) Range [%d, %d]', smin, args['mmax'])
            sys.exit(1)
        if not is_validip(args['master_ctl']):
            log.error('Please Enter Valid IP')
            sys.exit(1)

def ask_dpdk_mode():
    dpdk_mode = False
    while True:
        dpdk_mode = raw_input('Do you want to proceed Service Engine in DPDK Mode [y/\033[93mn\033[0m] ').lower() or 'n'
        if dpdk_mode in ['yes', 'no', 'y', 'n', 'ye']:
            break
    dpdk_mode = (dpdk_mode in ['yes', 'y', 'ye'])
    return dpdk_mode

def ask_inband_mode():
    inband_mgmt = False
    while True:
        inband_mgmt = raw_input('Enable inband management interface for this Service Engine (i.e. Use Management '
                                'interface for data traffic as well) [\033[0my/\033[93mn\033[0m] ').lower() or 'n'
        if inband_mgmt in ['yes', 'no', 'y', 'n', 'ye']:
            break
    inband_mgmt = (inband_mgmt in ['yes', 'y', 'ye'])
    return inband_mgmt

def ask_master_controller_ip():
    master_ctl = raw_input('Please enter the Controller IP for this SE to connect ')
    if not is_validip(master_ctl):
        log.error('Please Enter Valid IP')
        sys.exit(1)
    return master_ctl

def ask_controller_ip():
    cip = get_controller_ip()
    cip = raw_input('Please enter Controller IP (Default [\033[93m%s\033[0m]) ' % cip) or cip
    if not is_validip(cip):
        log.error('Controller IP %s is not valid', cip)
        sys.exit(1)
    return cip

def ask_controller_cpu(cmax):
    ccores = raw_input('Enter The Number Of Cores For AVI Controller. Range [4, \033[93m%d\033[0m] ' % cmax) or cmax
    ccores = int(ccores)
    if (ccores > cmax) or (ccores < 4):
        log.error('Please Enter Valid Number Within Range [4, %d]', cmax)
        sys.exit(1)
    return ccores

def ask_controller_memory(mmax):
    cmemg = raw_input('Please Enter Memory (in GB) for AVI Controller. Range [12, \033[93m%d\033[0m] ' % mmax) or mmax
    cmemg = int(cmemg)
    if (cmemg > mmax) or (cmemg < 4):
        log.error('Please Enter Memory (in GB) Range [%d, %d]', 4, (mmax/1024))
        sys.exit(1)
    return cmemg

def ask_se_cpu(cmax):
    scores = raw_input('Enter The Number Of Cores For AVI Service Engine. Range [1, \033[93m%d\033[0m] ' % cmax) or cmax
    scores = int(scores)
    if (scores > cmax) or (cmax < 1):
        log.error('Please Enter Valid Number Within Range [1, %d]', cmax)
        sys.exit(1)
    return scores

def ask_se_memory(scores, mmax):
    smin = scores * 1024
    if smin > mmax:
        log.error('Please Enter different number of cores as there isnt enough memory')
        sys.exit(1)
    smem = raw_input('Please Enter Memory (in MB) for AVI Service Engine. Range [%d, \033[93m%d\033[0m] ' % (smin, mmax)) or mmax
    smem = int(smem)
    if (smem > mmax) or (smem < smin):
        log.error('Please Enter Memory (in MB) Range [%d, %d]', smin, mmax)
        sys.exit(1)
    return smem

def ask_se_client_logs_disk():
    sdiskl = None
    sdiskl_path = raw_input('Do you have separate partition for AVI Service Engine Client Logs ? '
                            'If yes, please enter directory path, else leave it blank ') or None
    if sdiskl_path:
        sdiskl = raw_input('Please enter disk size (in GB) for AVI Service Engine Client Logs (Default [%dG]) ' % 5) or 5
        sdiskl = int(sdiskl)
    return sdiskl_path, sdiskl

def ask_se_system_logs_disk(largest_disk):
    sdisk_path = raw_input('Please enter directory path for AVI Service Engine System Logs (Default [%s]) ' %
                           os.path.join(largest_disk, 'opt/avi/se/data/')) or largest_disk
    sdisk = raw_input('Please enter disk size (in GB) for AVI Service Engine System Logs (Default [%dG]) ' % 10) or 10
    sdisk = int(sdisk)
    return sdisk_path, sdisk

def ask_controller_config_disk(largest_disk):
    cdisk_path = raw_input('Please enter directory path for AVI Controller Config (Default [%s]) ' %
                           os.path.join(largest_disk, 'opt/avi/controller/data/')) or largest_disk
    cdisk = raw_input('Please enter disk size (in GB) for AVI Controller Config (Default [%dG]) ' % 30) or 30
    cdisk = int(cdisk)
    return cdisk_path, cdisk

def ask_controller_metrics_disk():
    cdiskm = None
    cdiskm_path = raw_input('Do you have separate partition for AVI Controller Metrics ? '
                            'If yes, please enter directory path, else leave it blank ') or None
    if cdiskm_path:
        cdiskm = raw_input('Please enter disk size (in GB) for AVI Controller Metrics (Default [%dG]) ' % 30) or 30
        cdiskm = int(cdiskm)
    return cdiskm_path, cdiskm

def ask_controller_client_logs_disk():
    cdiskl = None
    cdiskl_path = raw_input('Do you have separate partition for AVI Controller Client Logs ? '
                            'If yes, please enter directory path, else leave it blank ') or None
    if cdiskl_path:
        cdiskl = raw_input('Please enter disk size (in GB) for AVI Controller Client Logs (Default [%dG]) ' % 40) or 40
        cdiskl = int(cdiskl)
    return cdiskl_path, cdiskl

def ask_ssh_sysint_ports():
    cssh = 5098
    csysint = 8443
    cssh = raw_input('Enter the Controller SSH port. (Default [%s]) ' % cssh) or cssh
    cssh = int(cssh)
    csysint = raw_input('Enter the Controller system-internal portal port. (Default [%s]) ' % csysint) or csysint
    csysint = int(csysint)
    return cssh, csysint

def ask_input(args):
    inp = raw_input('Do you want to run AVI Controller on this Host [\033[93my\033[0m/n] ').lower() or 'y'
    args['start_con'] = inp in ['y', 'yes']
    inp = raw_input('Do you want to run AVI SE on this Host [\033[0my/\033[93mn\033[0m] ').lower() or 'n'
    args['start_se'] = inp in ['y', 'yes']

    if args['start_con'] and args['start_se']:
        args['ccores'] = ask_controller_cpu(args['cmax']-1)
        args['cmemg'] = ask_controller_memory(args['mmax']/1024 - 1)
        args['cdisk_path'], args['cdisk'] = ask_controller_config_disk(args['largest_disk'])
        args['cdiskm_path'], args['cdiskm'] = ask_controller_metrics_disk()
        args['cdiskl_path'], args['cdiskl'] = ask_controller_client_logs_disk()
        args['cip'] = ask_controller_ip()

        left_cores = args['cmax'] - args['ccores']
        left_mem = args['mmax'] - args['cmemg']*1024

        args['scores'] = ask_se_cpu(left_cores)
        args['smem'] = ask_se_memory(args['scores'], left_mem)
        args['sdisk_path'], args['sdisk'] = ask_se_system_logs_disk(args['largest_disk'])
        args['sdiskl_path'], args['sdiskl'] = ask_se_client_logs_disk()
        args['master_ctl'] = ask_master_controller_ip()
        args['inband_mgmt'] = ask_inband_mode()
        args['dpdk_mode'] = ask_dpdk_mode()

    elif args['start_con'] and not args['start_se']:
        args['ccores'] = ask_controller_cpu(args['cmax'])
        args['cmemg'] = ask_controller_memory(args['mmax']/1024)
        args['cdisk_path'], args['cdisk'] = ask_controller_config_disk(args['largest_disk'])
        args['cdiskm_path'], args['cdiskm'] = ask_controller_metrics_disk()
        args['cdiskl_path'], args['cdiskl'] = ask_controller_client_logs_disk()
        args['cip'] = ask_controller_ip()

    elif not args['start_con'] and args['start_se']:
        args['scores'] = ask_se_cpu(args['cmax'])
        args['smem'] = ask_se_memory(args['scores'], args['mmax'])
        args['sdisk_path'], args['sdisk'] = ask_se_system_logs_disk(args['largest_disk'])
        args['sdiskl_path'], args['sdiskl'] = ask_se_client_logs_disk()
        args['master_ctl'] = ask_master_controller_ip()
        args['inband_mgmt'] = ask_inband_mode()
        args['dpdk_mode'] = ask_dpdk_mode()

    args['cssh'], args['csysint'] = ask_ssh_sysint_ports()
    return

def check_existing_containers(args):
    if args['start_con']:
        check_se_running_inband_dpdk_modes()
    if args['rm_containers']:
        return
    if args['start_se']:
        se_cont_exist = int(execute_command('docker ps | grep avinetworks/se: | grep -v CONTAINER | wc -l'))
        if se_cont_exist:
            while True:
                var = raw_input('\nAVI SE container is present, do you want to remove and continue [\033[93my\033[0m/n] ').lower() or 'y'
                if var in ['yes', 'no', 'y', 'n', 'ye']:
                    break
            if var in ['no', 'n']:
                sys.exit(1)
    if args['start_con']:
        con_cont_exist = int(execute_command('docker ps | grep avinetworks/controller: | grep -v CONTAINER | wc -l'))
        if con_cont_exist:
            while True:
                var = raw_input('\nAVI Controller container is present, do you want to remove and continue [\033[93my\033[0m/n] ').lower() or 'y'
                if var in ['yes', 'no', 'y', 'n', 'ye']:
                    break
            if var in ['no', 'n']:
                sys.exit(1)

def check_se_running_inband_dpdk_modes():
    if os.path.exists('/etc/systemd/system/avise.service'):
        with open('/etc/systemd/system/avise.service') as f:
            var = f.read()
            inband = ('-e SE_INBAND_MGMT=true ' in var)
            dpdk = ('-e DOCKERNETWORKMODE=HOST_DPDK ' in var)
            if inband and dpdk:
                log.error('Cannot create AVI controller since AVI ServiceEngine is running on host in dpdk and inband modes')
                sys.exit(1)

def disable_avi_services(use_systemd, start_se, start_con):
    log.info('Disabling AVI Services...')
    if use_systemd:
        if start_se:
            execute_command('systemctl stop avise')
            execute_command('systemctl disable avise.service')
        if start_con:
            execute_command('systemctl stop avicontroller')
            execute_command('systemctl disable avicontroller.service')
    else:
        if start_se:
            execute_command('service avise_watcher stop')
            for i in se_link :
                try:
                    os.unlink(i)
                except:
                    pass
        if start_con:
            log.info('Stopping avicontroller_watcher')
            execute_command('service avicontroller_watcher stop')
            for i in con_link:
                try:
                    os.unlink(i)
                except:
                    pass

def check_port_in_use(port, cssh, csysint, http, https):
    use = int(execute_command("netstat -natu | awk '{print $4}' | grep -w %s | wc -l" % port))
    if use:
        if port == cssh:
            port = raw_input('Port %s in use, enter available port for SSH ' % port)
        elif port == csysint:
            port = raw_input('Port %s in use, enter available port for portal ' % port)
        elif port == 5054:
            port = raw_input('Port %s in use, enter available port for Shell CLI ' % port)
        elif port == http:
            port = raw_input('Port %s in use, enter available port for HTTP ' % port)
        elif port == https:
            port = raw_input('Port %s in use, enter available port for HTTPS ' % port)
        elif port == 161:
            port = raw_input('Port %s in use, enter available port for SNMP walk ' % port)
        else:
            port = raw_input('Port %s in use, enter available port ' % port)
        port = int(port)
        port = check_port_in_use(port, cssh, csysint, http, https)

    return port

def _gen_portmap(cssh, csysint, start_con):
    s = ''
    http = 80
    https = 443
    for i in [cssh, csysint, 5054, http, https]:
        port = i
        if start_con:
            port = check_port_in_use(port, cssh, csysint, http, https)
        s += '-p %s:%s ' % (port, port)
        if i == cssh:
            cssh = port
        elif i == csysint:
            csysint = port
        elif i == http:
            http = port
        elif i == https:
            https = port
    # Add udp port mappings. Currently it is just 161
    for port in [161]:
        if start_con:
            port = check_port_in_use(port, cssh, csysint, http, https)
        s += '-p %s:%s/udp ' % (port, port)

    return (cssh, csysint, http, https, s)

def create_se_systemd_file(env_variables, mounts,master_ctl, se_image, dpdk_pre, dpdk_post):
    pre = ''
    post = ''
    if dpdk_pre:
        pre = 'ExecStartPre=-%s -c "%s"\n' % (bash_var, dpdk_pre)
    if dpdk_post:
        post = 'ExecStopPost=-%s -c "%s"\n' % (bash_var, dpdk_post)

    with open('/etc/systemd/system/avise.service', 'w') as f:
        f.write('[Unit]\n' +
                'Description=AVISE\n' +
                'After=docker.service\n' +
                'Requires=docker.service\n' +
                '\n' +
                '[Service]\n' +
                'TimeoutStartSec=0\n' +
                'Restart=always\n' +

                'ExecStartPre=-/usr/bin/docker rm -f avise\n' + pre +
                'ExecStartPre=/usr/bin/docker run --name=avise -d %s --net=host -v /mnt:/mnt -v /dev:/dev -v /etc/sysconfig/network-scripts:/etc/sysconfig/network-scripts -v /:/hostroot/ -v /etc/hostname:/etc/host_hostname -v /etc/localtime:/etc/localtime -v /var/run/docker.sock:/var/run/docker.sock %s -e CONTROLLERIP=%s -e CONTAINER_NAME=avise --privileged=true %s\n' % (env_variables, mounts, master_ctl, se_image) +
                'ExecStart=/usr/bin/docker wait avise\n' +
                'ExecStop=-%s -c "fstrim /proc/$(docker inspect --format=\'{{ .State.Pid }}\' avise)/root"\n' % bash_var +
                'ExecStop=-/usr/bin/docker stop -t 60 avise\n' +
                'ExecStopPost=-/usr/bin/docker rm -f avise\n' + post +
                '\n' +
                '[Install]\n' +
                'WantedBy=multi-user.target\n')

def create_controller_systemd_file(env_variables, mounts, con_image, devname, pmap):
    vipcmd = '/bin/bash -c "ip addr del $(ip addr | grep %s:1 | awk \'{print $2}\') dev %s"' % (devname, devname)
    with open('/etc/systemd/system/avicontroller.service', 'w') as f:
        f.write('[Unit]\n' +
                'Description=AVICONTROLLER\n' +
                'After=docker.service\n' +
                'Requires=docker.service\n' +
                '\n' +
                '[Service]\n' +
                'TimeoutStartSec=0\n' +
                'Restart=always\n' +

                'ExecStartPre=-/usr/bin/docker rm -f avicontroller\n' +
                'ExecStartPre=/usr/bin/docker run --name=avicontroller %s -d --privileged %s -v /:/hostroot/ -v /var/run/docker.sock:/var/run/docker.sock %s %s\n\n' % (pmap, env_variables, mounts, con_image) +
                'ExecStart=/usr/bin/docker wait avicontroller\n' +
                'ExecStop=-%s -c "fstrim /proc/$(docker inspect --format=\'{{ .State.Pid }}\' avicontroller)/root"\n' % bash_var +
                'ExecStop=-/usr/bin/docker stop avicontroller\n' +
                'ExecStopPost=-%s\n' % vipcmd +
                'ExecStopPost=-/usr/bin/docker rm -f avicontroller\n' +

                '\n' +
                '[Install]\n' +
                'WantedBy=multi-user.target\n')

def create_se_watcher_conf_file():
    with open('/etc/init.d/avise_watcher', 'w') as f:
        f.write("""
        #!/bin/sh
        #
        #       /etc/rc.d/init.d/avise_watcher
        #
        #       Daemon for avise_watcher
        #
        # chkconfig:   2345 99 99
        # description: AVISE WATCHER

        ### BEGIN INIT INFO
        # Provides:       avise_watcher
        # Required-Start: docker
        # Required-Stop:
        # Should-Start:
        # Should-Stop:
        # Default-Start: 2 3 4 5
        # Default-Stop:  0 1 6
        # Short-Description: start and stop avise_watcher
        # Description: AVISE Watcher
        ### END INIT INFO


        start() {
            echo "Starting avise watcher"
            nohup /etc/init.d/avise_watcher.sh 0<&- 1>/dev/null 2>&1 &
        }

        stop() {
            echo "Stopping avise watcher"
            ps -ef | grep avise_watcher.sh | grep -v grep | awk '{print $2}' | xargs --no-run-if-empty kill -9
            service avise stop
        }

        restart() {
            stop
            start
        }

        case "$1" in
            start)
            $1
            ;;
        stop)
            $1
            ;;
        restart)
            $1
            ;;
        status)
            docker ps -f name=avise
            ;;
        *)
            echo $"Usage: $0 {start|stop|status|restart}"
            exit 2
        esac

        exit $?
        """)
    execute_command('chmod u+x /etc/init.d/avise_watcher')

    with open('/etc/init.d/avise_watcher.sh', 'w') as f:
        f.write("""
        #!/bin/bash

        while true
        do
            service avise start
            sleep 1
            docker wait avise
            sleep 1
            docker rm -f avise
            sleep 1
        done
        """)
    execute_command('chmod u+x /etc/init.d/avise_watcher.sh')

    try:
        os.symlink('/etc/init.d/avise_watcher', '/etc/rc0.d/K99avise_watcher')
        os.symlink('/etc/init.d/avise_watcher', '/etc/rc1.d/K99avise_watcher')
        os.symlink('/etc/init.d/avise_watcher', '/etc/rc2.d/S99avise_watcher')
        os.symlink('/etc/init.d/avise_watcher', '/etc/rc3.d/S99avise_watcher')
        os.symlink('/etc/init.d/avise_watcher', '/etc/rc4.d/S99avise_watcher')
        os.symlink('/etc/init.d/avise_watcher', '/etc/rc5.d/S99avise_watcher')
        os.symlink('/etc/init.d/avise_watcher', '/etc/rc5.d/K99avise_watcher')
    except:
        pass

def create_se_conf_file(env_variables, mounts, master_ctl, se_image, dpdk_pre, dpdk_post):
    with open('/etc/init.d/avise', 'w') as f:
        f.write("""
        #!/bin/sh
        #
        #       /etc/rc.d/init.d/avise
        #
        #       Daemon for avise
        #
        # chkconfig:   2345 99 99
        # description: AVI SE

        ### BEGIN INIT INFO
        # Provides:       avise
        # Required-Start: docker
        # Required-Stop:
        # Should-Start:
        # Should-Stop:
        # Default-Start: 2 3 4 5
        # Default-Stop:  0 1 6
        # Short-Description: start and stop avise
        # Description: AVISE
        ### END INIT INFO


        prestart() {
            echo "prestart avise"
            %s
        }

        start() {
            prestart
            echo "Starting avise"
            /usr/bin/docker run --name=avise -d %s --net=host -v /mnt:/mnt  -v /dev:/dev -v /etc/sysconfig/network-scripts:/etc/sysconfig/network-scripts -v /:/hostroot/ -v /etc/hostname:/etc/host_hostname -v /etc/localtime:/etc/localtime -v /var/run/docker.sock:/var/run/docker.sock %s -e NON_SYSTEMD=1 -e CONTROLLERIP=%s -e CONTAINER_NAME=avise --privileged=true %s
        }

        stop() {
            echo "Stopping avise"
            /usr/bin/docker stop -t 60 avise
            /usr/bin/docker rm -f avise
            %s
        }

        restart() {
            stop
            start
        }

        case "$1" in
        start)
            $1
            ;;
        stop)
            $1
            ;;
        restart)
            $1
            ;;
        status)
            status avise
            ;;
        *)
            echo $"Usage: $0 {start|stop|status|restart}"
            exit 2
        esac

        exit $?
        """ % (dpdk_pre, env_variables, mounts, master_ctl, se_image, dpdk_post))

    execute_command('chmod u+x /etc/init.d/avise')
    create_se_watcher_conf_file()

def create_controller_watcher_conf_file():
    with open('/etc/init.d/avicontroller_watcher', 'w') as f:
        f.write("""
        #!/bin/sh
        #
        #       /etc/rc.d/init.d/avicontroller_watcher
        #
        #       Daemon for avicontroller_watcher
        #
        # chkconfig:   2345 99 99
        # description: AVICONTROLLER WATCHER

        ### BEGIN INIT INFO
        # Provides:       avicontroller_watcher
        # Required-Start: docker
        # Required-Stop:
        # Should-Start:
        # Should-Stop:
        # Default-Start: 2 3 4 5
        # Default-Stop:  0 1 6
        # Short-Description: start and stop avicontroller_watcher
        # Description: avicontroller Watcher
        ### END INIT INFO


        start() {
            echo "Starting avicontroller watcher"
            check_running=`ps -aef | grep avicontroller_watcher.sh| grep -v grep | wc -l`
            if [ $check_running -ne 0 ]; then
                echo "avicontroller watcher is already running"
                exit 0
            fi
            nohup /etc/init.d/avicontroller_watcher.sh 0<&- 1>/dev/null 2>&1 &
        }

        stop() {
            echo "Stopping avicontroller watcher"
            ps -ef | grep avicontroller_watcher.sh | grep -v grep | awk '{print $2}' | xargs --no-run-if-empty kill -9
            service avicontroller stop
        }

        restart() {
            stop
            start
        }

        case "$1" in
        start)
            $1
            ;;
        stop)
            $1
            ;;
        restart)
            $1
            ;;
        status)
            docker ps -f name=avicontroller
            ;;
        *)
            echo $"Usage: $0 {start|stop|status|restart}"
            exit 2
        esac

        exit $?
        """)
    execute_command('chmod u+x /etc/init.d/avicontroller_watcher')

    with open('/etc/init.d/avicontroller_watcher.sh', 'w') as f:
        f.write("""
        #!/bin/bash

        while true
        do
            service avicontroller start
            sleep 1
            docker wait avicontroller
            sleep 1
            docker rm -f avicontroller
            sleep 1
        done
        """)
    execute_command('chmod u+x /etc/init.d/avicontroller_watcher.sh')

    try:
        os.symlink('/etc/init.d/avicontroller_watcher', '/etc/rc0.d/K99avicontroller_watcher')
        os.symlink('/etc/init.d/avicontroller_watcher', '/etc/rc1.d/K99avicontroller_watcher')
        os.symlink('/etc/init.d/avicontroller_watcher', '/etc/rc2.d/S99avicontroller_watcher')
        os.symlink('/etc/init.d/avicontroller_watcher', '/etc/rc3.d/S99avicontroller_watcher')
        os.symlink('/etc/init.d/avicontroller_watcher', '/etc/rc4.d/S99avicontroller_watcher')
        os.symlink('/etc/init.d/avicontroller_watcher', '/etc/rc5.d/S99avicontroller_watcher')
        os.symlink('/etc/init.d/avicontroller_watcher', '/etc/rc5.d/K99avicontroller_watcher')
    except:
        pass

def create_controller_conf_file(env_variables, mounts, con_image, devname, pmap):
    vipcmd = '/bin/bash -c "ip addr del $(ip addr | grep %s:1 | awk \'{print $2}\') dev %s"' % (devname, devname)
    with open('/etc/init.d/avicontroller', 'w') as f:
        f.write("""
        #!/bin/sh
        #
        #       /etc/rc.d/init.d/avicontroller
        #
        #       Daemon for avicontroller
        #
        # chkconfig:   2345 99 99
        # description: AVI SE

        ### BEGIN INIT INFO
        # Provides:       avicontroller
        # Required-Start: docker
        # Required-Stop:
        # Should-Start:
        # Should-Stop:
        # Default-Start: 2 3 4 5
        # Default-Stop:  0 1 6
        # Short-Description: start and stop avicontroller
        # Description: AVICONTROLLER
        ### END INIT INFO


        start() {
            echo "Starting avicontroller"
            /usr/bin/docker run --name=avicontroller %s -d --privileged %s -e NON_SYSTEMD=1 -v /:/hostroot/ -v /var/run/docker.sock:/var/run/docker.sock %s %s
        }

        stop() {
            echo "Stopping avicontroller"
            /usr/bin/docker stop  avicontroller
            /usr/bin/docker rm -f avicontroller
            %s
        }

        restart() {
            stop
            start
        }

        case "$1" in
        start)
            $1
            ;;
        stop)
            $1
            ;;
        restart)
            $1
            ;;
        status)
            status avicontroller
            ;;
        *)
        echo $"Usage: $0 {start|stop|status|restart}"
        exit 2
        esac

        exit $?
        """ % (pmap, env_variables, mounts, con_image, vipcmd))

    execute_command('chmod u+x /etc/init.d/avicontroller')
    create_controller_watcher_conf_file()

def centos_specific(dist, start_se):
    if dist.find('centos') != -1:
        if start_se:
            with open('/etc/sysconfig/network-scripts/ifcfg-avi_eth0', 'w') as f:
                f.write('TYPE=Ethernet\n' +
                        'BOOTPROTO=none\n' +
                        'ONBOOT=no\n' +
                        'NAME=avi_eth0\n' +
                        'DEVICE=avi_eth0\n')
            with open('/etc/sysconfig/network-scripts/ifcfg-avi_eth1', 'w') as f:
                f.write('TYPE=Ethernet\n' +
                        'BOOTPROTO=none\n' +
                        'ONBOOT=no\n' +
                        'NAME=avi_eth1\n' +
                        'DEVICE=avi_eth1\n')
            with open('/etc/sysconfig/network-scripts/ifcfg-avi_eth2', 'w') as f:
                f.write('TYPE=Ethernet\n' +
                        'BOOTPROTO=none\n' +
                        'ONBOOT=no\n' +
                        'NAME=avi_eth2\n' +
                        'DEVICE=avi_eth2\n')
            with open('/etc/sysconfig/network-scripts/ifcfg-avi_eth3', 'w') as f:
                f.write('TYPE=Ethernet\n' +
                        'BOOTPROTO=none\n' +
                        'ONBOOT=no\n' +
                        'NAME=avi_eth3\n' +
                        'DEVICE=avi_eth3\n')
            with open('/etc/sysconfig/network-scripts/ifcfg-avi_eth4', 'w') as f:
                f.write('TYPE=Ethernet\n' +
                        'BOOTPROTO=none\n' +
                        'ONBOOT=no\n' +
                        'NAME=avi_eth4\n' +
                        'DEVICE=avi_eth4\n')
            with open('/etc/sysconfig/network-scripts/ifcfg-avi_eth5', 'w') as f:
                f.write('TYPE=Ethernet\n' +
                        'BOOTPROTO=none\n' +
                        'ONBOOT=no\n' +
                        'NAME=avi_eth5\n' +
                        'DEVICE=avi_eth5\n')
            with open('/etc/sysconfig/network-scripts/ifcfg-avi_eth6', 'w') as f:
                f.write('TYPE=Ethernet\n' +
                        'BOOTPROTO=none\n' +
                        'ONBOOT=no\n' +
                        'NAME=avi_eth6\n' +
                        'DEVICE=avi_eth6\n')
            with open('/etc/sysconfig/network-scripts/ifcfg-avi_eth7', 'w') as f:
                f.write('TYPE=Ethernet\n' +
                        'BOOTPROTO=none\n' +
                        'ONBOOT=no\n' +
                        'NAME=avi_eth7\n' +
                        'DEVICE=avi_eth7\n')
            with open('/etc/sysconfig/network-scripts/ifcfg-avi_eth8', 'w') as f:
                f.write('TYPE=Ethernet\n' +
                        'BOOTPROTO=none\n' +
                        'ONBOOT=no\n' +
                        'NAME=avi_eth8\n' +
                        'DEVICE=avi_eth8\n')

def check_systemd(dist, dist_ver):
    use_systemd = False
    if (dist.find('oel') != -1) and (dist_ver.find('7.0') != -1):
        use_systemd = True
    elif (dist.find('oel') != -1) and (dist_ver.find('7.1') != -1):
        use_systemd = True
    elif (dist.find('oel') != -1) and (dist_ver.find('7.2') != -1):
        use_systemd = True
    elif (dist.find('oel') != -1) and (dist_ver.find('7.3') != -1):
        use_systemd = True
    elif (dist.find('oel') != -1) and (dist_ver.find('7.4') != -1):
        use_systemd = True
    elif (dist.find('rhel') != -1) and (dist_ver.find('7.0') != -1):
        use_systemd = True
    elif (dist.find('rhel') != -1) and (dist_ver.find('7.1') != -1):
        use_systemd = True
    elif (dist.find('rhel') != -1) and (dist_ver.find('7.2') != -1):
        use_systemd = True
    elif (dist.find('rhel') != -1) and (dist_ver.find('7.3') != -1):
        use_systemd = True
    elif (dist.find('rhel') != -1) and (dist_ver.find('7.4') != -1):
        use_systemd = True
    elif (dist.find('centos') != -1) and (dist_ver.find('7.0') != -1):
        use_systemd = True
    elif (dist.find('centos') != -1) and (dist_ver.find('7.1') != -1):
        use_systemd = True
    elif (dist.find('centos') != -1) and (dist_ver.find('7.2') != -1):
        use_systemd = True
    elif (dist.find('centos') != -1) and (dist_ver.find('7.3') != -1):
        use_systemd = True
    elif (dist.find('centos') != -1) and (dist_ver.find('7.4') != -1):
        use_systemd = True
    elif (dist.find('ubuntu') != -1) and (dist_ver.find('16.04') != -1):
        use_systemd = True
    return use_systemd

def setup(args):
    check_prereq()
    dist,dist_ver = check_dist_ver()
    #check_pkg(args['start_se'], args['start_con'])
    se_image = con_image = None
    senv_variables = env_variables = None
    smounts = mounts = ''

    if args['start_con']:
        ccon = 'Yes'
    else:
        ccon = 'No'

    if args['start_se']:
        scon = 'Yes'
    else:
        scon = 'No'

    log.info('Run SE           : \033[93m%s\033[0m', scon)
    if args['start_se']:
        log.info('SE Cores         : \033[93m%d\033[0m', args['scores'])
        log.info('Memory(MB)       : \033[93m%d\033[0m', args['smem'])
        if args['sdisk']:
            log.info('Disk(GB)         : \033[93m%d\033[0m', args['sdisk'])
    log.info('Run Controller   : \033[93m%s\033[0m', ccon)
    if args['start_con']:
        log.info('Controller Cores : \033[93m%d\033[0m', args['ccores'])
        log.info('Memory(GB)       : \033[93m%d\033[0m', args['cmemg'])
        if args['cdisk']:
            log.info('Disk(GB)         : \033[93m%d\033[0m', args['cdisk'])

    if args['start_con']:
        log.info('Controller IP    : \033[93m%s\033[0m', args['cip'])
    if args['start_se']:
        log.info('Controller IP this SE will communicate with: \033[93m%s\033[0m', args['master_ctl'])

    use_systemd = check_systemd(dist, dist_ver)
    disable_avi_services(use_systemd, args['start_se'], args['start_con'])
    args['cssh'], args['csysint'], http, https, pmap = _gen_portmap(args['cssh'], args['csysint'], args['start_con'])

    if args['start_se']:
        se_image = cleanup_images_se()
    if args['start_con']:
        con_image = cleanup_images_con()

    centos_specific(dist, args['start_se'])
    dpdk_pre = ''
    dpdk_post = ''
    if args['start_se']:
        if args['dpdk_mode']:
            dpdk_pre = "modprobe uio; " \
                       "mkdir -p /mnt/huge; umount /mnt/huge; rm /mnt/huge/* ; " \
                       "mount -t hugetlbfs nodev /mnt/huge"
            dpdk_post = "rmmod igb_uio; rmmod rte_kni; umount /mnt/huge"

    gen_variables = '-e CNTRL_SSH_PORT=%d -e SYSINT_PORT=%d -e HTTP_PORT=%d -e HTTPS_PORT=%d' % (args['cssh'], args['csysint'], http, https)
    if args['start_se']:
        senv_variables = '-e NTHREADS=%d -e SEMEMMB=%d ' % (args['scores'], args['smem'])
        if args['dpdk_mode']:
            senv_variables += ' -e DOCKERNETWORKMODE=HOST_DPDK '
        else:
            senv_variables += ' -e DOCKERNETWORKMODE=HOST '
        if args['sdisk_path']:
            args['sdisk_path'] = os.path.join(args['sdisk_path'], 'opt/avi/se/data')
            smounts += ' -v %s:/vol/ ' % args['sdisk_path']
            senv_variables += ' -e DISKSZ=%d ' % (args['sdisk']*1024)
            execute_command('mkdir -p %s' % args['sdisk_path'])
            execute_command('rm -rf %s/*' % args['sdisk_path'])
        if args['sdiskl_path']:
            args['sdiskl_path'] = os.path.join(args['sdiskl_path'], 'opt/avi/se/logs')
            smounts += ' -v %s:/vol_logs/ ' % args['sdiskl_path']
            senv_variables += ' -e LOG_DISKSZ=%d ' % (args['sdiskl']*1024)
            execute_command('mkdir -p %s' % args['sdiskl_path'])
            execute_command('rm -rf %s/*' % args['sdiskl_path'])
        senv_variables += gen_variables
        if args['inband_mgmt']:
            senv_variables += ' -e SE_INBAND_MGMT=true '
        else:
            senv_variables += ' -e SE_INBAND_MGMT=false '

    devname = 'eth0'
    if args['start_con']:
        env_variables = '-e "CONTAINER_NAME=avicontroller" -e "MANAGEMENT_IP=%s" -e NUM_CPU=%d ' \
                        '-e NUM_MEMG=%d ' % (args['cip'], args['ccores'], args['cmemg'])
        if args['cdisk_path']:
            args['cdisk_path'] = os.path.join(args['cdisk_path'], 'opt/avi/controller/data')
            mounts += ' -v %s:/vol/ ' % args['cdisk_path']
            env_variables += ' -e DISK_GB=%d ' % args['cdisk']
            execute_command('mkdir -p %s' % args['cdisk_path'])
            execute_command('rm -rf %s/*' % args['cdisk_path'])
        if args['cdiskm_path']:
            args['cdiskm_path'] = os.path.join(args['cdiskm_path'], 'opt/avi/controller/metrics')
            mounts += ' -v %s:/vol_metrics/ ' % args['cdiskm_path']
            env_variables += ' -e METRICS_DISK_GB=%d ' % args['cdiskm']
            execute_command('mkdir -p %s' % args['cdiskm_path'])
            execute_command('rm -rf %s/*' % args['cdiskm_path'])
        if args['cdiskl_path']:
            args['cdiskl_path'] = os.path.join(args['cdiskl_path'], 'opt/avi/controller/logs')
            mounts += ' -v %s:/vol_logs/ ' % args['cdiskl_path']
            env_variables += ' -e LOGS_DISK_GB=%d ' % args['cdiskl']
            execute_command('mkdir -p %s' % args['cdiskl_path'])
            execute_command('rm -rf %s/*' % args['cdiskl_path'])
        env_variables += gen_variables

        devname = get_management_devname(args['cip'])

    execute_command('sysctl -w kernel.core_pattern=/var/crash/%e.%p.%t.core')
    if use_systemd:
        if args['start_se']:
            create_se_systemd_file(senv_variables, smounts, args['master_ctl'], se_image, dpdk_pre, dpdk_post)
            execute_command('systemctl enable avise.service')
        if args['start_con']:
            create_controller_systemd_file(env_variables, mounts, con_image, devname, pmap)
            execute_command('systemctl enable avicontroller.service')
            if args['setup_json']:
                execute_command('cp -f %s %s/setup.json' % (args['setup_json'], args['cdisk_path']))

        log.info('Installation Successful. Starting Services..')

        if args['start_con']:
            execute_command('systemctl start avicontroller ')
        if args['start_se']:
            execute_command('systemctl start avise')
    else:
        execute_command('chkconfig docker on')

        if args['start_se']:
            create_se_conf_file(senv_variables, smounts, args['master_ctl'], se_image, dpdk_pre, dpdk_post)
            execute_command('chkconfig avise_watcher on')
        if args['start_con']:
            create_controller_conf_file(env_variables, mounts, con_image, devname, pmap)
            execute_command('chkconfig avicontroller on')
            if args['setup_json']:
                execute_command('cp -f %s %s/setup.json' % (args['setup_json'], args['cdisk_path']))

        log.info('Installation Successful. Starting Services..')

        if args['start_con']:
            execute_command('service avicontroller_watcher start')
        if args['start_se']:
            execute_command('service avise_watcher start')

def adjust_mmax_cmax(args):
    if args.get('dpdk_mode'):
        if args['mmax'] < 10*1024:
            log.error('Please Install AVI on Host with >= 10GB')
            sys.exit(1)
        if args['cmax'] <= 4 :
            log.error('Please Install AVI on Host with > 4 Cores')
            sys.exit(1)
    if args['mmax'] > (256*1024):
        args['mmax'] = 256*1024
    if args['cmax'] > 128:
        args['cmax'] = 128
    return

def setup_log():
    global log
    try:
	p = subprocess.Popen('mkdir -p /opt/avi', stdout=subprocess.PIPE, stderr=subprocess.PIPE, shell=True)
	out, err = p.communicate()
	if err:
	    print("Error in creating directory '/opt/avi' output: %s, error: %s" % (out, err))
            sys.exit(1)
        log = logging.getLogger('bm_setup')
        log.setLevel(logging.DEBUG)
        fh = logging.FileHandler('/opt/avi/avi_setup.log')
        fh.setLevel(logging.DEBUG)
        formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')
        fh.setFormatter(formatter)
        log.addHandler(fh)
        st = logging.StreamHandler()
        st.setLevel(logging.INFO)
        log.addHandler(st)
    except Exception as e:
        print('%s exception in creating logfile %s', e, traceback.format_exc())
        sys.exit(1)

def _get_largest_disk():
    val = execute_command('df -lT | tail -n +2 | egrep -v "tmpfs|overlay" | sort -k 5,5 -nr | awk  \'NR==1{print $NF}\'')
    log.info('Found disk with largest capacity at [\033[93m%s\033[0m]' % val)
    return val.strip()

def get_controller_ip():
    cip = execute_command("ip route get 1 | awk '{print $NF; exit}'")
    if not cip:
        log.error('Controller interface not found')
        sys.exit(1)
    return cip

def get_cmax_mmax():
    cmax = int(execute_command('cat /proc/cpuinfo | grep processor | wc -l'))
    mmax = int(execute_command('cat /proc/meminfo | grep MemTotal | sed s/[A-Za-z:' ']//g'))
    mmax /= 1024
    return cmax, mmax

if __name__ == "__main__":
    if os.geteuid() != 0:
        print('This script must be run with root privileges..')
        sys.exit(1)
    args = dict()
    setup_log()
    check_prereq()
    args['largest_disk'] = _get_largest_disk()
    args['cmax'], args['mmax'] = get_cmax_mmax()
    args['setup_json'] = None
    args['rm_containers'] = False
    if len(sys.argv) == 1:
        banner()
        ask_input(args)
    else:
        parser = argparse.ArgumentParser(description='AVI baremetal setup')
        parser.add_argument('-d', '--dpdk_mode', action='store_true', required=False,
                            default=False, help='Run SE in DPDK Mode. Default is False')
        parser.add_argument('-s', '--run_se', action='store_true',  required=False,
                            default=False, help='Run SE locally. Default is False')
        parser.add_argument('-sc', '--se_cores', required=False, type=int,
                            default=1, help='Cores to be used for AVI SE. Default is %d' % 1)
        parser.add_argument('-sm', '--se_memory_mb', required=False, type=int,
                            default=2048, help='Memory to be used for AVI SE. Default is %d MB' % 2048)
        parser.add_argument('-sdp', '--se_disk_path', required=False, type=str,
                            help='Directory path to be used for AVI SE system Logs. Default is %s' %
                                 os.path.join(args['largest_disk'], 'opt/avi/se/data/'), default=args['largest_disk'])
        parser.add_argument('-sd', '--se_disk_gb', required=False, type=int, default=10,
                            help='Disk size (in GB) to be used for AVI SE system Logs. Default is %d' % 10)
        parser.add_argument('-sldp', '--se_logs_disk_path', required=False, type=str,
                            help='Directory path to be used for AVI SE client Logs.')
        parser.add_argument('-sld', '--se_logs_disk_gb', required=False, type=int,
                            help='Disk size (in GB) to be used for AVI SE Client Logs.')
        parser.add_argument('-sim', '--se_inband_mgmt', action='store_true',  required=False,
                            default=False, help='Inband management enabled (i.e Use management interface for '
                                                'data traffic as well). Default is False')
        parser.add_argument('-c', '--run_controller', action='store_true',  required=False,
                            default=False, help='Run Controller locally. Default is No')
        parser.add_argument('-cc', '--con_cores', required=False, type=int,
                            default=4, help='Cores to be used for AVI Controller. Default is %d' % 4)
        parser.add_argument('-cm', '--con_memory_gb', required=False, type=int,
                            default=12, help='Memory to be used for AVI Controller. Default is %d' % 12)
        parser.add_argument('-cdp', '--con_disk_path', required=False, type=str,
                            help='Directory Path to be used for AVI Controller Config Data. Default is %s' %
                                 os.path.join(args['largest_disk'], 'opt/avi/controller/data/'), default=args['largest_disk'])
        parser.add_argument('-cd', '--con_disk_gb', required=False, type=int, default=30,
                            help='Disk size (in GB) to be used for AVI Controller Config Data. Default is %d GB' % 30)
        parser.add_argument('-cmdp', '--con_metrics_disk_path', required=False, type=str,
                            help='Directory Path to be used for AVI Controller Metrics DB.')
        parser.add_argument('-cmd', '--con_metrics_disk_gb', required=False, type=int,
                            help='Disk size (in GB) to be used for AVI Controller Metrics DB.')
        parser.add_argument('-cldp', '--con_logs_disk_path', required=False, type=str,
                            help='Directory Path to be used for AVI Controller Client Logs.')
        parser.add_argument('-cld', '--con_logs_disk_gb', required=False, type=int,
                            help='Disk size (in GB) to be used for AVI Controller Client Logs.')
        parser.add_argument('-i', '--controller_ip', required=False,
                            default=None, help='Controller IP Address')
        parser.add_argument('-m', '--master_ctl_ip', required=False,
                            default=None, help='Controller IP this SE will communicate with')
        parser.add_argument('-sj', '--setup-json', required=False,
                            default=None, help='Controller initial configuration (setup.json) absolute file path')
        parser.add_argument('-cssh', required=False, type=int, default=5098,
                            help='SSH port for Controller (Default 5098)')
        parser.add_argument('-sysint', required=False, type=int, default=8443,
                            help='System-internal port for Controller (Default 8443)')
        parser.add_argument('-rmc', '--rm_containers', action='store_true',  required=False,
                            default=False, help='Remove AVI containers if any')
        arg = parser.parse_args()
        args['start_se'] = arg.run_se
        args['scores'] = arg.se_cores
        args['smem'] = arg.se_memory_mb
        args['start_con'] = arg.run_controller
        args['ccores'] = arg.con_cores
        args['cmemg'] = arg.con_memory_gb
        args['cip'] = arg.controller_ip
        args['master_ctl'] = arg.master_ctl_ip
        args['cdisk_path'] = arg.con_disk_path
        args['cdisk'] = arg.con_disk_gb if arg.con_disk_gb else 0
        args['sdisk_path'] = arg.se_disk_path
        args['sdisk'] = arg.se_disk_gb if arg.se_disk_gb else 0
        args['cdiskm_path'] = arg.con_metrics_disk_path if arg.con_metrics_disk_path else None
        args['cdiskm'] = arg.con_metrics_disk_gb if arg.con_metrics_disk_gb else None
        args['cdiskl_path'] = arg.con_logs_disk_path if arg.con_logs_disk_path else None
        args['cdiskl'] = arg.con_logs_disk_gb if arg.con_logs_disk_gb else None
        args['sdiskl_path'] = arg.se_logs_disk_path if arg.se_logs_disk_path else None
        args['sdiskl'] = arg.se_logs_disk_gb if arg.se_logs_disk_gb else None
        args['dpdk_mode'] = arg.dpdk_mode
        args['setup_json'] = arg.setup_json
        args['cssh'] = arg.cssh
        args['csysint'] = arg.sysint
        args['inband_mgmt'] = arg.se_inband_mgmt
        args['rm_containers'] = arg.rm_containers

    if args['start_con'] and not args['cip']:
        args['cip'] = get_controller_ip()
    adjust_mmax_cmax(args)
    validate_input(args)
    log.debug('args: %s', args)
    setup(args)

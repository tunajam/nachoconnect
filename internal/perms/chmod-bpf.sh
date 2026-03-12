#!/bin/bash
# NachoConnect ChmodBPF — sets /dev/bpf* permissions on macOS
# Runs at boot via LaunchDaemon and on first install.
# Based on the Wireshark ChmodBPF approach.

BPF_GROUP="access_bpf"

# Ensure group exists
/usr/sbin/dseditgroup -o read "$BPF_GROUP" > /dev/null 2>&1 || /usr/sbin/dseditgroup -o create "$BPF_GROUP"

# Apply permissions to all BPF devices
chgrp "$BPF_GROUP" /dev/bpf*
chmod g+rw /dev/bpf*

exit 0

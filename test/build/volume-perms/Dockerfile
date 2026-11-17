FROM alpine
VOLUME /vol/subvol
# At this point, the directory should exist, with default permissions 0755, the
# contents below /vol/subvol should be frozen, and we shouldn't get an error
# from trying to write to it because we it was created automatically.
RUN dd if=/dev/zero bs=512 count=1 of=/vol/subvol/subvolfile

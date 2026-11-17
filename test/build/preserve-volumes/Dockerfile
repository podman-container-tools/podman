FROM alpine
RUN mkdir -p /vol/subvol/subsubvol
RUN dd if=/dev/zero bs=512 count=1 of=/vol/subvol/subsubvol/subsubvolfile
VOLUME /vol/subvol
# At this point, the contents below /vol/subvol should be frozen.
RUN dd if=/dev/zero bs=512 count=1 of=/vol/subvol/subvolfile
# In particular, /vol/subvol/subvolfile should be wiped out.
RUN dd if=/dev/zero bs=512 count=1 of=/vol/volfile
# However, /vol/volfile should exist.
VOLUME /vol
# And this should be redundant.
VOLUME /vol/subvol
# And now we've frozen /vol.
RUN dd if=/dev/zero bs=512 count=1 of=/vol/anothervolfile
# Which means that in the image we're about to commit, /vol/anothervolfile
# shouldn't exist, either.

# ADD files which should persist.
ADD Dockerfile /vol/Dockerfile
RUN stat /vol/Dockerfile
ADD Dockerfile /vol/Dockerfile2
RUN stat /vol/Dockerfile2

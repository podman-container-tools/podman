####> This option file is used in:
####>   podman build, farm build
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--compat-volumes**

Handle directories marked using the VOLUME instruction (both in this build, and
those inherited from base images) such that their contents can only be modified
by ADD and COPY instructions. Any changes made in those locations by RUN
instructions will be reverted. Before the introduction of this option, this
behavior was the default, but it is now disabled by default.

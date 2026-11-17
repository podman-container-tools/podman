#### **--filter**, **-f**=*filter*

Filter events that are displayed.  They must be in the format of "filter=value".  The following
filters are supported:

| **Filter** | **Description**                     |
|------------|-------------------------------------|
| container  | [Name or ID] Container's name or ID |
| event      | event_status (described above)      |
| image      | [Name or ID] Image name or ID       |
| label      | [key] or [key=value] label          |
| pod        | [Name or ID] Pod name or ID         |
| type       | Event type (e.g., container, image, pod, volume, network) |
| volume     | [Name or ID] Volume name or ID      |

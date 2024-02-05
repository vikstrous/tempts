# tstemporal
Type-safe Temporal Go SDK wrapper


List of guarantees provided by this wrapper:

Workers:

* Have all the right activities and workflows registered before starting

Activities:

* Are called on the right namespace and queue
* Are called with the right parameter types
* Return the right response types
* Registered functions match the right type signature

Workflows:

* Are called on the right namespace and queue
* Are called with the right parameter types
* Return the right response types
* Registered functions match the right type signature

Schedules:

* Set the right workflow argument types
* Can be "set" on start up of the application and the intended effect will be applied to the state of the schedule on the cluster automatically

Queries and updates:

* Are called with the right types
* Return the right types
* Registered functions match the right type signature

More guarantees coming soon:

* Automatic fixture based tests
* Signals type safety
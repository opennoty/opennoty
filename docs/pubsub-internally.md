Real-time information about all subscribers is stored in mongodb.
It uses mongodb's watch to detect changes and cache them in memory.
When a PUBLISH occurs, it is sent to the nodes that are subscribed to that topic.

package schema

const PayloadSchema = `{
    "$id": "docs/spec/sourcemaps/sourcemap-metadata.json",
    "title": "Sourcemap Metadata",
    "description": "Sourcemap Metadata",
    "type": "object",
    "properties": {
        "bundle_filepath": {
            "description": "relative path of the minified bundle file",
            "type": "string",
            "maxLength": 1024,
            "minLength": 1
        },
        "service_version": {
            "description": "Version of the service emitting this event",
            "type": "string",
            "maxLength": 1024,
            "minLength": 1
        },
        "service_name": {
            "description": "Immutable name of the service emitting this event",
            "type": "string",
            "pattern": "^[a-zA-Z0-9 _-]+$",
            "maxLength": 1024,
            "minLength": 1
        }
    },
    "required": ["bundle_filepath", "service_name", "service_version"]
}
`

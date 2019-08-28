## what
upload a target directory to cloudflare workers kv
```
key: name-of-file
value: {
    content (base64)
    contentType
}
```

## run
```
CF_API_ACCOUNT_ID=your-account-id \
CF_API_KEY=your-api-key \
CF_API_EMAIL=your-email \ 
CF_KV_NAMESPACE=namespace-to-use \
TARGET_DIRECTORY=your-target-directory (relative) \
go run index.go
```
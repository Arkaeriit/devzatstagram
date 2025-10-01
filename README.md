# Devzatstagram

A Devzat plugin to add in-chat image sharing support.

![In-chat use](https://devzatstagram.bobignou.red/static/in-chat-use.png)

## Running it

To run it on your Devzat instance, simply build it and run it with a JSON config file as argument.

```
./devzatstagram config.json
```

## Configuration

The application requires a JSON configuration file to be passed as a command-line argument. Below are the available configuration fields:

| Field                 | Description                                              | Default Value                                          |
|-----------------------|----------------------------------------------------------|--------------------------------------------------------|
| `MaxStorageSize`      | Maximum total storage size in bytes                      | `1073741824` (1 GiB)                                   |
| `MaxFileSize`         | Maximum individual file size in bytes                    | `268435456` (256 MiB)                                  |
| `FileKeepingDuration` | Duration to keep files before automatic cleanup          | `"10m"`                                                |
| `StoragePath`         | Directory path where uploaded files are stored           | `"./storage"`                                          |
| `DevzatToken`         | Authentication token for Devzat integration (required)   | *No default - must be provided*                        |
| `DevzatHost`          | Devzat server host and port                              | `"devzat.hackclub.com:5556"`                           |
| `WebHost`             | Public URL of the web server                             | Constructed from `WebPort`: `"http://localhost:8080"`  |
| `WebPort`             | Port number for the web server                           | `8080`                                                 |
| `Debug`               | Enable debug mode (verbose logging)                      | `false`                                                |

### Example Configuration

```json
{
  "DevzatToken": "your-token-here",
  "MaxStorageSize": 2147483648,
  "MaxFileSize": 536870912,
  "FileKeepingDuration": "20m",
  "WebPort": 3000,
  "Debug": true
}
```

### Configuration Notes

- **DevzatToken** is required and the application will exit if not provided
- If `WebHost` is not specified, it will automatically be constructed as `http://localhost:{WebPort}`
- If you want to use a custom domain or external URL, explicitly set `WebHost` in your configuration
- `FileKeepingDuration` uses Go's duration string format: valid units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`
  - Examples: `"10m"` (10 minutes), `"1h30m"` (1 hour 30 minutes), `"30s"` (30 seconds)

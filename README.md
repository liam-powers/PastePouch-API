### PastePouch - Back End

This is the Go back end for the PastePouch application. Gin support is coming soon to handle HTTP requests and will feature user authentication to integrate with the frontend and CLI.

All of the functionality currently lives in main.go. To interact with SQL, you'll need to run a PostgreSQL database and to change the connection information in the string at the top of func main(). Also ensure it's open to new connections.

If you are unfamiliar with the project, check out the description of it on [my website](https://liampowers.me).

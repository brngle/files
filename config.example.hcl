http {
  url    = "https://files.example.com"
  bind   = "localhost:3333"
  secret = "yeet420"
}

volume "personal" {
  path     = "/mnt/personal"
  privacy  = "unlisted"
  features = ["compress"]
}

volume "sharex" {
  path     = "/mnt/personal/sharex"
  privacy  = "unlisted"
  features = ["sharex", "compress"]
}

volume "media" {
  path     = "/mnt/media"
  features = ["transcode", "compress"]
}

discord {
  guild_id      = "1122316261595033700"
  client_id     = ""
  client_secret = ""
  token         = ""
}

role "admin" {
  user_ids = ["my-discord-user-id"]
  admin    = true
}
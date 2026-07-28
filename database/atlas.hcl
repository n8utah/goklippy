env "local" {
  src = "file://schema.sql"

  migration {
    dir = "file://migrations"
  }

  dev = "docker://postgres/16/dev"
}

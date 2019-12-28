
provider "google" {
  credentials = file("~/.secret/xecus-circleci.json")

  project = "abeja-intern-project-2019"
  region = "us-central1"
  zone = "us-central1-a"
}

resource "google_compute_network" "feature-db-test-network" {
  name = "feature-db-test"
}

resource "google_compute_subnetwork" "feature-db-test" {
  name = "feature-db-test"
  ip_cidr_range = "10.30.0.0/16"
  network = google_compute_network.feature-db-test-network.name
  description = "feature-db-test"
  region = "us-central1"
}

resource "google_compute_firewall" "feature-db-test" {
  name = "feature-db-test"
  network = google_compute_network.feature-db-test-network.name

  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "tcp"
    ports = [
      "22",
      "80",
      "443"]
  }

  target_tags = [
    "feature-db-test-calcnode-1",
    "feature-db-test-calcnode-2",
    "feature-db-test-proxynode-1"
  ]

  source_ranges = ["x.x.x.x/32"]
}

resource "google_compute_instance" "calcnode-1" {
  name = "feature-db-test-calcnode-1"
  machine_type = "f1-micro"
  zone = "us-central1-a"

  boot_disk {
    initialize_params {
      image = "ubuntu-1804-lts"
    }
  }

  // Local SSD disk
//  scratch_disk {
//    interface = "SCSI"
//  }

  network_interface {
    network = "feature-db-test"

    access_config {
      // Ephemeral IP
    }
    subnetwork = google_compute_subnetwork.feature-db-test.name

  }
  //  metadata_startup_script = "echo hi > /test.txt"

//  service_account {
//    scopes = [
//      "userinfo-email",
//      "compute-ro",
//      "storage-ro"]
//  }

  tags = [
    "feature-db-test-calcnode-1"
  ]
}

resource "google_compute_instance" "calcnode-2" {
  name = "feature-db-calcnode-2"
  machine_type = "f1-micro"
  zone = "us-central1-a"

  boot_disk {
    initialize_params {
      image = "ubuntu-1804-lts"
    }
  }

  // Local SSD disk
//  scratch_disk {
//    interface = "SCSI"
//  }

  network_interface {
    network = "feature-db-test"

    access_config {
      // Ephemeral IP
    }
  }
  //  metadata_startup_script = "echo hi > /test.txt"

//  service_account {
//    scopes = [
//      "userinfo-email",
//      "compute-ro",
//      "storage-ro"]
//  }

  tags = [
    "feature-db-test-calcnode-2"
  ]
}


resource "google_compute_instance" "proxynode-1" {
  name = "feature-db-proxynode-1"
  machine_type = "f1-micro"
  zone = "us-central1-a"

  boot_disk {
    initialize_params {
      image = "ubuntu-1804-lts"
    }
  }

  // Local SSD disk
//  scratch_disk {
//    interface = "SCSI"
//  }

  network_interface {
    network = "feature-db-test"

    access_config {
      // Ephemeral IP
    }
  }
  //  metadata_startup_script = "echo hi > /test.txt"

//  service_account {
//    scopes = [
//      "userinfo-email",
//      "compute-ro",
//      "storage-ro"]
//  }

  tags = [
    "feature-db-test-proxynode-1"]
}
wit_bindgen::generate!({
    world: "exercise",
    path: "wit",
    generate_all,
});

struct Component;

impl Guest for Component {
    fn test_fs_set_size() -> String {
        use wasi::filesystem::preopens::get_directories;
        use wasi::filesystem::types::{DescriptorFlags, OpenFlags, PathFlags};

        let dirs = get_directories();
        if dirs.is_empty() {
            return "no preopened directories".into();
        }
        let (dir, _) = &dirs[0];

        let file = match dir.open_at(
            PathFlags::SYMLINK_FOLLOW,
            "test-set-size.txt",
            OpenFlags::CREATE | OpenFlags::TRUNCATE,
            DescriptorFlags::READ | DescriptorFlags::WRITE,
        ) {
            Ok(f) => f,
            Err(e) => return format!("open_at failed: {e:?}"),
        };

        match file.write(&[b'A'; 20], 0) {
            Ok(n) if n == 20 => {}
            Ok(n) => return format!("write returned {n}, expected 20"),
            Err(e) => return format!("write failed: {e:?}"),
        }

        if let Err(e) = file.set_size(5) {
            return format!("set_size failed: {e:?}");
        }

        match file.stat() {
            Ok(stat) if stat.size == 5 => "ok".into(),
            Ok(stat) => format!("expected size 5, got {}", stat.size),
            Err(e) => format!("stat failed: {e:?}"),
        }
    }

    fn test_fs_metadata_hash() -> String {
        use wasi::filesystem::preopens::get_directories;
        use wasi::filesystem::types::{DescriptorFlags, OpenFlags, PathFlags};

        let dirs = get_directories();
        if dirs.is_empty() {
            return "no preopened directories".into();
        }
        let (dir, _) = &dirs[0];

        let file = match dir.open_at(
            PathFlags::SYMLINK_FOLLOW,
            "test-hash.txt",
            OpenFlags::CREATE | OpenFlags::TRUNCATE,
            DescriptorFlags::READ | DescriptorFlags::WRITE,
        ) {
            Ok(f) => f,
            Err(e) => return format!("open_at failed: {e:?}"),
        };

        let hash1 = match file.metadata_hash() {
            Ok(h) => h,
            Err(e) => return format!("metadata_hash 1 failed: {e:?}"),
        };
        let hash2 = match file.metadata_hash() {
            Ok(h) => h,
            Err(e) => return format!("metadata_hash 2 failed: {e:?}"),
        };

        if hash1.lower != hash2.lower || hash1.upper != hash2.upper {
            return format!(
                "hashes differ: ({},{}) vs ({},{})",
                hash1.lower, hash1.upper, hash2.lower, hash2.upper
            );
        }
        if hash1.lower == 0 && hash1.upper == 0 {
            return "hash is all zeros".into();
        }
        "ok".into()
    }

    fn test_fs_is_same_object() -> String {
        use wasi::filesystem::preopens::get_directories;
        use wasi::filesystem::types::{DescriptorFlags, OpenFlags, PathFlags};

        let dirs = get_directories();
        if dirs.is_empty() {
            return "no preopened directories".into();
        }
        let (dir, _) = &dirs[0];

        // Create the file first
        let _ = match dir.open_at(
            PathFlags::SYMLINK_FOLLOW,
            "test-same.txt",
            OpenFlags::CREATE | OpenFlags::TRUNCATE,
            DescriptorFlags::READ | DescriptorFlags::WRITE,
        ) {
            Ok(f) => f,
            Err(e) => return format!("create failed: {e:?}"),
        };

        let file1 = match dir.open_at(
            PathFlags::SYMLINK_FOLLOW,
            "test-same.txt",
            OpenFlags::empty(),
            DescriptorFlags::READ,
        ) {
            Ok(f) => f,
            Err(e) => return format!("open 1 failed: {e:?}"),
        };

        let file2 = match dir.open_at(
            PathFlags::SYMLINK_FOLLOW,
            "test-same.txt",
            OpenFlags::empty(),
            DescriptorFlags::READ,
        ) {
            Ok(f) => f,
            Err(e) => return format!("open 2 failed: {e:?}"),
        };

        if !file1.is_same_object(&file2) {
            return "same file not detected as same object".into();
        }
        "ok".into()
    }
}

export!(Component);

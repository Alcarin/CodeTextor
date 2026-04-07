<?php
/**
 * User Controller for hybrid testing.
 * This file contains PHP, SQL strings, HTML and JS.
 */
class UserController {
    public function getProfile($id) {
        // SQL query inside a string
        $query = "SELECT id, username, email FROM users WHERE id = $id LIMIT 1";
        $db->execute($query);
        return "Profile data";
    }

    public function deleteUser($id) {
        $sql = "DELETE FROM users WHERE id = :id";
        $this->db->query($sql, ['id' => $id]);
    }
}

function render_header() {
    echo "<h1>Welcome to the Hybrid Test</h1>";
}
?>

<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Mixed Content</title>
    <style>
        .profile-card { border: 1px solid #ccc; padding: 10px; }
    </style>
</head>
<body>
    <?php render_header(); ?>
    
    <div class="profile-card" id="user-profile">
        <p>Loading user profile...</p>
    </div>

    <script>
        // Inline JavaScript
        function initApp() {
            console.log("App initialized");
            fetch('/api/profile').then(r => r.json()).then(data => {
                document.getElementById('user-profile').innerText = data.name;
            });
        }
        window.onload = initApp;
    </script>
</body>
</html>

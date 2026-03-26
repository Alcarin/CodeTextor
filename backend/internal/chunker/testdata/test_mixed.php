
<html>
<head><title>Test</title></head>
<body>
    <h1>Benvenuto</h1>
    <?php
    echo "<div>Contenuto PHP dinamico</div>";
    
    function test_function() {
        echo "<script>console.log('JS inside PHP');</script>";
        return true;
    }
    ?>
    <script>
        console.log("JS outside PHP");
    </script>
</body>
</html>

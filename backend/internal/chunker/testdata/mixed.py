import sqlite3

class DatabaseManager:
    def __init__(self, db_path):
        self.conn = sqlite3.connect(db_path)

    def create_tables(self):
        # Mixed language: SQL inside triple-quoted string
        query = """
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY,
            username TEXT NOT NULL,
            email TEXT UNIQUE
        );
        """
        self.conn.execute(query)

    def add_user(self, username, email):
        sql = "INSERT INTO users (username, email) VALUES (?, ?)"
        self.conn.execute(sql, (username, email))

def main():
    mgr = DatabaseManager(":memory:")
    mgr.create_tables()

if __name__ == "__main__":
    main()

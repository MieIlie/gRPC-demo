# Demo Testing Accounts

Use these pre-seeded credentials to test user registration, login, and realtime messaging locally.

| Username | Password      | Display Name    | Purpose                        |
|----------|---------------|-----------------|--------------------------------|
| `alice`  | `password123` | Alice Henderson | Active sender / receiver test |
| `bob`    | `password123` | Bob Vance       | Active sender / receiver test |

## DB Schema Pre-seeding

These accounts are inserted automatically during database initialization by the [init-db.sql](file:///C:/Me/j2ee/personal_proj/init-db.sql) script:

```sql
INSERT INTO users (id, username, password_hash, display_name) VALUES
('11111111-1111-1111-1111-111111111111', 'alice', '$2a$10$6fxT3J2ic0JFeCES6HPaze67je.5CgRLOZ2rWye4k9Cb43Mct4koK', 'Alice Henderson'),
('22222222-2222-2222-2222-222222222222', 'bob', '$2a$10$6fxT3J2ic0JFeCES6HPaze67je.5CgRLOZ2rWye4k9Cb43Mct4koK', 'Bob Vance')
ON CONFLICT (username) DO NOTHING;
```

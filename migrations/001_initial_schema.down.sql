-- Drop tables in reverse order of dependencies

-- Drop purchases table
DROP TABLE IF EXISTS purchases;

-- Drop checkout_attempts table
DROP TABLE IF EXISTS checkout_attempts;

-- Drop items table
DROP TABLE IF EXISTS items;

-- Drop sales table
DROP TABLE IF EXISTS sales;
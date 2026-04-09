#include <iostream>
#include <vector>

namespace Geometry {
    class Shape {
    public:
        virtual double area() = 0;
    };

    class Circle : public Shape {
    private:
        double radius;
    public:
        Circle(double r) : radius(r) {}
        double area() override {
            return 3.14 * radius * radius;
        }
    };
}

int main() {
    Geometry::Circle c(5.0);
    std::cout << "Area: " << c.area() << std::endl;
    // FIXME: use more precise PI
    return 0;
}
